package apmhttp

import (
	"net/http"
	"strconv"
)

// StatusCodeString returns the stringified status code. Prefer this to
// strconv.Itoa to avoid allocating memory for well known status codes.
func StatusCodeString(statusCode int) string {
	switch statusCode {
	case http.StatusContinue:
		return "100"
	case http.StatusSwitchingProtocols:
		return "101"
	case http.StatusProcessing:
		return "102"

	case http.StatusOK:
		return "200"
	case http.StatusCreated:
		return "201"
	case http.StatusAccepted:
		return "202"
	case http.StatusNonAuthoritativeInfo:
		return "203"
	case http.StatusNoContent:
		return "204"
	case http.StatusResetContent:
		return "205"
	case http.StatusPartialContent:
		return "206"
	case http.StatusMultiStatus:
		return "207"
	case http.StatusAlreadyReported:
		return "208"
	case http.StatusIMUsed:
		return "226"

	case http.StatusMultipleChoices:
		return "300"
	case http.StatusMovedPermanently:
		return "301"
	case http.StatusFound:
		return "302"
	case http.StatusSeeOther:
		return "303"
	case http.StatusNotModified:
		return "304"
	case http.StatusUseProxy:
		return "305"

	case http.StatusTemporaryRedirect:
		return "307"
	case http.StatusPermanentRedirect:
		return "308"

	case http.StatusBadRequest:
		return "400"
	case http.StatusUnauthorized:
		return "401"
	case http.StatusPaymentRequired:
		return "402"
	case http.StatusForbidden:
		return "403"
	case http.StatusNotFound:
		return "404"
	case http.StatusMethodNotAllowed:
		return "405"
	case http.StatusNotAcceptable:
		return "406"
	case http.StatusProxyAuthRequired:
		return "407"
	case http.StatusRequestTimeout:
		return "408"
	case http.StatusConflict:
		return "409"
	case http.StatusGone:
		return "410"
	case http.StatusLengthRequired:
		return "411"
	case http.StatusPreconditionFailed:
		return "412"
	case http.StatusRequestEntityTooLarge:
		return "413"
	case http.StatusRequestURITooLong:
		return "414"
	case http.StatusUnsupportedMediaType:
		return "415"
	case http.StatusRequestedRangeNotSatisfiable:
		return "416"
	case http.StatusExpectationFailed:
		return "417"
	case http.StatusTeapot:
		return "418"
	case http.StatusUnprocessableEntity:
		return "422"
	case http.StatusLocked:
		return "423"
	case http.StatusFailedDependency:
		return "424"
	case http.StatusUpgradeRequired:
		return "426"
	case http.StatusPreconditionRequired:
		return "428"
	case http.StatusTooManyRequests:
		return "429"
	case http.StatusRequestHeaderFieldsTooLarge:
		return "431"
	case http.StatusUnavailableForLegalReasons:
		return "451"

	case http.StatusInternalServerError:
		return "500"
	case http.StatusNotImplemented:
		return "501"
	case http.StatusBadGateway:
		return "502"
	case http.StatusServiceUnavailable:
		return "503"
	case http.StatusGatewayTimeout:
		return "504"
	case http.StatusHTTPVersionNotSupported:
		return "505"
	case http.StatusVariantAlsoNegotiates:
		return "506"
	case http.StatusInsufficientStorage:
		return "507"
	case http.StatusLoopDetected:
		return "508"
	case http.StatusNotExtended:
		return "510"
	case http.StatusNetworkAuthenticationRequired:
		return "511"
	}
	return strconv.Itoa(statusCode)
}
