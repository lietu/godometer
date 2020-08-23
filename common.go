package godometer

const APITimeLayout = "2006-01-02 15:04"

type UpdateDataPoint struct {
	Timestamp         string  `json:"ts"`
	Meters            float32 `json:"m"`
	MetersPerSecond   float32 `json:"mps"`
	KilometersPerHour float32 `json:"kph"`
}

type APIRow struct {
	Meters            float32 `json:"m"`
	MetersPerSecond   float32 `json:"mps"`
	KilometersPerHour float32 `json:"kph"`
}

type UpdateStatsRequest struct {
	DataPoints []UpdateDataPoint `json:"dataPoints"`
}
