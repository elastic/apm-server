package beater

// import "github.com/elastic/beats/libbeat/monitoring"

// type EntityMetrics struct {
// 	registry        *monitoring.Registry
// 	validationCount *monitoring.Int
// 	validationError *monitoring.Int
// 	transformations *monitoring.Int
// }

// var errorMetricsReg = monitoring.Default.NewRegistry("apm-server.entity.error")

// var ErrorMetrics = EntityHandlingMetrics{
// 	registry:        errorMetricsReg,
// 	transformations: monitoring.NewInt(errorMetricsReg, "transformations"),
// 	validationTotal: monitoring.NewInt(errorMetricsReg, "validation.total"),
// 	validationError: monitoring.NewInt(errorMetricsReg, "validation.errors"),
// }

// var transactionMetricsReg = monitoring.Default.NewRegistry("apm-server.entity.transaction")

// var TransactionMetrics = EntityHandlingMetrics{
// 	registry:        transactionMetricsReg,
// 	transformations: monitoring.NewInt(transactionMetricsReg, "transformations"),
// 	validationTotal: monitoring.NewInt(transactionMetricsReg, "validation.total"),
// 	validationError: monitoring.NewInt(transactionMetricsReg, "validation.errors"),
// }

// // var errorMetrics =

// // var validationCount =

// // validationError = monitoring.NewInt(errorMetrics, "validation.errors")
// // transformations = monitoring.NewInt(errorMetrics, "transformations")
