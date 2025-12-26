package prediction

import (
	"fmt"
	"math"
	"time"
)

// HoltWinters implements the Holt-Winters exponential smoothing method.
// Also known as Triple Exponential Smoothing, it handles:
// - Level (baseline value)
// - Trend (increasing/decreasing pattern)
// - Seasonality (repeating patterns)
//
// The algorithm uses three smoothing equations:
// Level:    L_t = α(Y_t - S_{t-m}) + (1-α)(L_{t-1} + T_{t-1})
// Trend:    T_t = β(L_t - L_{t-1}) + (1-β)T_{t-1}
// Seasonal: S_t = γ(Y_t - L_t) + (1-γ)S_{t-m}
//
// Forecast: F_{t+h} = L_t + h*T_t + S_{t-m+h}
//
// This is a classical 1960s algorithm - no machine learning required!
type HoltWinters struct {
	config *Config

	// Fitted model state
	level     float64
	trend     float64
	seasonals []float64

	// Smoothing parameters (optimized or user-specified)
	alpha float64
	beta  float64
	gamma float64

	// Historical data and fitted values
	data         []float64
	timestamps   []time.Time
	fittedValues []float64
	residuals    []float64

	// Is the model fitted?
	fitted bool
}

// NewHoltWinters creates a new Holt-Winters predictor with default settings
func NewHoltWinters() *HoltWinters {
	return NewHoltWintersWithConfig(DefaultConfig())
}

// NewHoltWintersWithConfig creates a Holt-Winters predictor with custom config
func NewHoltWintersWithConfig(config *Config) *HoltWinters {
	return &HoltWinters{
		config: config,
		alpha:  config.Alpha,
		beta:   config.Beta,
		gamma:  config.Gamma,
	}
}

// Name returns the predictor's method name
func (hw *HoltWinters) Name() string {
	return "Holt-Winters"
}

// Fit trains the model on historical data
func (hw *HoltWinters) Fit(data []float64) error {
	return hw.FitWithTimestamps(data, nil)
}

// FitWithTimestamps trains the model with timestamp information
func (hw *HoltWinters) FitWithTimestamps(data []float64, timestamps []time.Time) error {
	m := hw.config.SeasonalPeriod
	minRequired := m * hw.config.MinDataPoints

	if len(data) < minRequired {
		return fmt.Errorf("insufficient data: need at least %d points (%d seasons), got %d",
			minRequired, hw.config.MinDataPoints, len(data))
	}

	hw.data = make([]float64, len(data))
	copy(hw.data, data)

	if timestamps != nil {
		hw.timestamps = make([]time.Time, len(timestamps))
		copy(hw.timestamps, timestamps)
	}

	// Initialize components
	hw.initializeComponents(data)

	// Optimize parameters if not specified
	if hw.config.Alpha == 0 || hw.config.Beta == 0 || hw.config.Gamma == 0 {
		hw.optimizeParameters(data)
	} else {
		hw.alpha = hw.config.Alpha
		hw.beta = hw.config.Beta
		hw.gamma = hw.config.Gamma
	}

	// Fit the model
	hw.fitModel(data)

	hw.fitted = true
	return nil
}

// initializeComponents sets up initial level, trend, and seasonal values
func (hw *HoltWinters) initializeComponents(data []float64) {
	m := hw.config.SeasonalPeriod

	// Initial level: average of first season
	var sum float64
	for i := 0; i < m; i++ {
		sum += data[i]
	}
	hw.level = sum / float64(m)

	// Initial trend: average change between first two seasons
	if len(data) >= 2*m {
		var trendSum float64
		for i := 0; i < m; i++ {
			trendSum += (data[m+i] - data[i]) / float64(m)
		}
		hw.trend = trendSum / float64(m)
	} else {
		hw.trend = 0
	}

	// Initial seasonals
	hw.seasonals = make([]float64, m)
	if hw.config.SeasonalType == SeasonalAdditive {
		// Additive: seasonal = observation - level
		for i := 0; i < m; i++ {
			hw.seasonals[i] = data[i] - hw.level
		}
	} else {
		// Multiplicative: seasonal = observation / level
		for i := 0; i < m; i++ {
			if hw.level != 0 {
				hw.seasonals[i] = data[i] / hw.level
			} else {
				hw.seasonals[i] = 1.0
			}
		}
	}

	// Normalize seasonals to sum to 0 (additive) or average to 1 (multiplicative)
	hw.normalizeSeasonals()
}

// normalizeSeasonals ensures seasonal components are properly normalized
func (hw *HoltWinters) normalizeSeasonals() {
	m := len(hw.seasonals)
	if m == 0 {
		return
	}

	var sum float64
	for _, s := range hw.seasonals {
		sum += s
	}

	if hw.config.SeasonalType == SeasonalAdditive {
		// Adjust so they sum to 0
		avg := sum / float64(m)
		for i := range hw.seasonals {
			hw.seasonals[i] -= avg
		}
	} else {
		// Adjust so they average to 1
		avg := sum / float64(m)
		if avg != 0 {
			for i := range hw.seasonals {
				hw.seasonals[i] /= avg
			}
		}
	}
}

// optimizeParameters finds optimal smoothing parameters using grid search
func (hw *HoltWinters) optimizeParameters(data []float64) {
	bestAlpha, bestBeta, bestGamma := 0.2, 0.1, 0.1
	bestError := math.MaxFloat64

	// Grid search over parameter space
	for alpha := 0.1; alpha <= 0.9; alpha += 0.1 {
		for beta := 0.01; beta <= 0.5; beta += 0.05 {
			for gamma := 0.01; gamma <= 0.5; gamma += 0.05 {
				hw.alpha = alpha
				hw.beta = beta
				hw.gamma = gamma

				// Reset and initialize
				hw.initializeComponents(data)

				// Calculate error
				err := hw.calculateSSE(data)
				if err < bestError {
					bestError = err
					bestAlpha = alpha
					bestBeta = beta
					bestGamma = gamma
				}
			}
		}
	}

	hw.alpha = bestAlpha
	hw.beta = bestBeta
	hw.gamma = bestGamma

	// Re-initialize with best parameters
	hw.initializeComponents(data)
}

// calculateSSE calculates Sum of Squared Errors for parameter optimization
func (hw *HoltWinters) calculateSSE(data []float64) float64 {
	m := hw.config.SeasonalPeriod
	n := len(data)

	level := hw.level
	trend := hw.trend
	seasonals := make([]float64, len(hw.seasonals))
	copy(seasonals, hw.seasonals)

	var sse float64

	for t := m; t < n; t++ {
		// Forecast
		var forecast float64
		seasonIdx := t % m
		if hw.config.SeasonalType == SeasonalAdditive {
			forecast = level + trend + seasonals[seasonIdx]
		} else {
			forecast = (level + trend) * seasonals[seasonIdx]
		}

		// Error
		error := data[t] - forecast
		sse += error * error

		// Update components
		prevLevel := level
		if hw.config.SeasonalType == SeasonalAdditive {
			level = hw.alpha*(data[t]-seasonals[seasonIdx]) + (1-hw.alpha)*(level+trend)
			trend = hw.beta*(level-prevLevel) + (1-hw.beta)*trend
			seasonals[seasonIdx] = hw.gamma*(data[t]-level) + (1-hw.gamma)*seasonals[seasonIdx]
		} else {
			if seasonals[seasonIdx] != 0 {
				level = hw.alpha*(data[t]/seasonals[seasonIdx]) + (1-hw.alpha)*(level+trend)
			}
			trend = hw.beta*(level-prevLevel) + (1-hw.beta)*trend
			if level != 0 {
				seasonals[seasonIdx] = hw.gamma*(data[t]/level) + (1-hw.gamma)*seasonals[seasonIdx]
			}
		}
	}

	return sse
}

// fitModel fits the model and calculates fitted values and residuals
func (hw *HoltWinters) fitModel(data []float64) {
	m := hw.config.SeasonalPeriod
	n := len(data)

	hw.fittedValues = make([]float64, n)
	hw.residuals = make([]float64, n)

	level := hw.level
	trend := hw.trend
	seasonals := make([]float64, len(hw.seasonals))
	copy(seasonals, hw.seasonals)

	for t := 0; t < n; t++ {
		seasonIdx := t % m

		// Calculate fitted value
		var fitted float64
		if hw.config.SeasonalType == SeasonalAdditive {
			fitted = level + trend + seasonals[seasonIdx]
		} else {
			fitted = (level + trend) * seasonals[seasonIdx]
		}

		hw.fittedValues[t] = fitted
		hw.residuals[t] = data[t] - fitted

		// Update components (only after first season to avoid initialization artifacts)
		if t >= m-1 {
			prevLevel := level
			if hw.config.SeasonalType == SeasonalAdditive {
				level = hw.alpha*(data[t]-seasonals[seasonIdx]) + (1-hw.alpha)*(level+trend)
				trend = hw.beta*(level-prevLevel) + (1-hw.beta)*trend
				seasonals[seasonIdx] = hw.gamma*(data[t]-level) + (1-hw.gamma)*seasonals[seasonIdx]
			} else {
				if seasonals[seasonIdx] != 0 {
					level = hw.alpha*(data[t]/seasonals[seasonIdx]) + (1-hw.alpha)*(level+trend)
				}
				trend = hw.beta*(level-prevLevel) + (1-hw.beta)*trend
				if level != 0 {
					seasonals[seasonIdx] = hw.gamma*(data[t]/level) + (1-hw.gamma)*seasonals[seasonIdx]
				}
			}
		}
	}

	// Save final state for forecasting
	hw.level = level
	hw.trend = trend
	hw.seasonals = seasonals
}

// Predict generates forecasts for n periods ahead
func (hw *HoltWinters) Predict(horizons int) (*ForecastResult, error) {
	if !hw.fitted {
		return nil, fmt.Errorf("model not fitted: call Fit() first")
	}

	if horizons < 1 {
		return nil, fmt.Errorf("horizons must be at least 1, got %d", horizons)
	}

	m := hw.config.SeasonalPeriod
	n := len(hw.data)

	forecasts := make([]Forecast, horizons)

	// Calculate residual standard error for prediction intervals
	residualStdErr := hw.calculateResidualStdErr()

	// Determine last timestamp
	var lastTimestamp time.Time
	var hasTimestamps bool
	if len(hw.timestamps) > 0 {
		lastTimestamp = hw.timestamps[len(hw.timestamps)-1]
		hasTimestamps = true
	}

	for h := 1; h <= horizons; h++ {
		// Forecast value
		var value float64
		seasonIdx := (n + h - 1) % m

		if hw.config.TrendType == TrendDamped {
			// Damped trend
			dampedTrend := hw.trend * hw.dampingSum(h)
			if hw.config.SeasonalType == SeasonalAdditive {
				value = hw.level + dampedTrend + hw.seasonals[seasonIdx]
			} else {
				value = (hw.level + dampedTrend) * hw.seasonals[seasonIdx]
			}
		} else {
			// Standard trend
			if hw.config.SeasonalType == SeasonalAdditive {
				value = hw.level + float64(h)*hw.trend + hw.seasonals[seasonIdx]
			} else {
				value = (hw.level + float64(h)*hw.trend) * hw.seasonals[seasonIdx]
			}
		}

		// Prediction interval (approximate)
		// Standard error increases with horizon
		z := 1.96 // 95% confidence
		intervalWidth := z * residualStdErr * math.Sqrt(float64(h))

		// Timestamp for forecast
		var ts time.Time
		if hasTimestamps && len(hw.timestamps) >= 2 {
			interval := hw.timestamps[1].Sub(hw.timestamps[0])
			ts = lastTimestamp.Add(time.Duration(h) * interval)
		}

		forecasts[h-1] = Forecast{
			Timestamp:       ts,
			Value:           value,
			LowerBound:      value - intervalWidth,
			UpperBound:      value + intervalWidth,
			ConfidenceLevel: hw.config.ConfidenceLevel,
			HorizonIndex:    h,
		}
	}

	// Calculate error metrics
	metrics := hw.calculateErrorMetrics()

	return &ForecastResult{
		Forecasts:          forecasts,
		FittedValues:       hw.fittedValues,
		Residuals:          hw.residuals,
		Parameters:         hw.getParameters(),
		Metrics:            metrics,
		SeasonalComponents: hw.seasonals,
		TrendComponents:    []float64{hw.trend},
		Method:             hw.Name(),
	}, nil
}

// FitPredict combines fitting and predicting in one step
func (hw *HoltWinters) FitPredict(data []float64, horizons int) (*ForecastResult, error) {
	if err := hw.Fit(data); err != nil {
		return nil, err
	}
	return hw.Predict(horizons)
}

// dampingSum calculates the sum of damping factors for h periods
func (hw *HoltWinters) dampingSum(h int) float64 {
	phi := hw.config.DampingFactor
	if phi >= 1 {
		return float64(h)
	}

	// Sum: phi + phi^2 + ... + phi^h = phi * (1 - phi^h) / (1 - phi)
	return phi * (1 - math.Pow(phi, float64(h))) / (1 - phi)
}

// calculateResidualStdErr calculates the standard error of residuals
func (hw *HoltWinters) calculateResidualStdErr() float64 {
	if len(hw.residuals) < 2 {
		return 0
	}

	var sumSq float64
	for _, r := range hw.residuals {
		sumSq += r * r
	}

	// Degrees of freedom: n - number of parameters
	df := float64(len(hw.residuals) - 3) // Subtract for alpha, beta, gamma
	if df < 1 {
		df = 1
	}

	return math.Sqrt(sumSq / df)
}

// calculateErrorMetrics computes forecast accuracy metrics
func (hw *HoltWinters) calculateErrorMetrics() *ErrorMetrics {
	n := len(hw.residuals)
	if n == 0 {
		return &ErrorMetrics{}
	}

	var sumAbs, sumSq, sumAPE, sumSAPE float64
	validCount := 0

	for i, r := range hw.residuals {
		absR := math.Abs(r)
		sumAbs += absR
		sumSq += r * r

		// MAPE and SMAPE require non-zero actual values
		if hw.data[i] != 0 {
			sumAPE += absR / math.Abs(hw.data[i])
			sumSAPE += absR / (math.Abs(hw.data[i]) + math.Abs(hw.fittedValues[i]))
			validCount++
		}
	}

	mae := sumAbs / float64(n)
	mse := sumSq / float64(n)
	rmse := math.Sqrt(mse)

	var mape, smape float64
	if validCount > 0 {
		mape = (sumAPE / float64(validCount)) * 100
		smape = (sumSAPE / float64(validCount)) * 200
	}

	return &ErrorMetrics{
		MAE:   mae,
		MSE:   mse,
		RMSE:  rmse,
		MAPE:  mape,
		SMAPE: smape,
	}
}

// getParameters returns the fitted model parameters
func (hw *HoltWinters) getParameters() *ModelParameters {
	return &ModelParameters{
		Alpha:            hw.alpha,
		Beta:             hw.beta,
		Gamma:            hw.gamma,
		SeasonalPeriod:   hw.config.SeasonalPeriod,
		InitialLevel:     hw.level,
		InitialTrend:     hw.trend,
		InitialSeasonals: hw.seasonals,
		SeasonalType:     hw.config.SeasonalType,
		TrendType:        hw.config.TrendType,
	}
}

// GetLevel returns the current level component
func (hw *HoltWinters) GetLevel() float64 {
	return hw.level
}

// GetTrend returns the current trend component
func (hw *HoltWinters) GetTrend() float64 {
	return hw.trend
}

// GetSeasonals returns the seasonal components
func (hw *HoltWinters) GetSeasonals() []float64 {
	result := make([]float64, len(hw.seasonals))
	copy(result, hw.seasonals)
	return result
}

// IsFitted returns whether the model has been fitted
func (hw *HoltWinters) IsFitted() bool {
	return hw.fitted
}
