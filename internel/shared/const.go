package shared

const (
	CDK = "X-PARAM-CDK"
	RID = "X-PARAM-RID"
	SID = "X-PARAM-SID"

	ApiPrefix = "/sync"

	REQ  = 0
	LST  = 1
	SIG  = 4
	DONE = 5
	SYN  = 8
	FIN  = 9
	ERR  = 2
)

func GetTypeName(t int32) string {
	switch t {
	case REQ:
		return "REQ"
	case LST:
		return "LST"
	case SIG:
		return "SIG"
	case DONE:
		return "DONE"
	case SYN:
		return "SYN"
	case FIN:
		return "FIN"
	case ERR:
		return "ERR"
	default:
		return "UNKNOWN"
	}
}
