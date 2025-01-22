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

type ResourceInfo struct {
	Path  string
	Error string
}

type DiffFileInfo struct {
	Path string
	Attr int32
}

type PendingFileInfo struct {
	Path    string
	Attr    int32
	Exist   bool
	ModTime int64
}

type IFileInfo struct {
	Attr    int32
	ModTime int64
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
