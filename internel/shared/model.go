package shared

const (
	TFile = iota + 1
	TDir
	TEmptyDir
)

type Params struct {
	CDK string
	SID string
	RID string
}
type DiffFileInfo struct {
	Path string
	Attr int32
}

type PendingFileInfo struct {
	Path  string
	Attr  int32
	Exist bool
}

type ServerSession struct {
	Id    string
	Param *Params
	List  []PendingFileInfo
}

type ClientSession struct {
	Id   string
	Done bool
	List []PendingFileInfo
}
