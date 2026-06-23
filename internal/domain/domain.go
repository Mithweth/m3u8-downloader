package domain

type PlaylistType int

const (
	PlaylistMaster PlaylistType = iota
	PlaylistTS
	PlaylistFMP4
)

type Segment struct {
	URL      string
	Duration float64
}

type Playlist struct {
	Type     PlaylistType
	Segments []Segment
}

type DownloadEvent struct {
	Done          int
	Total         int
	AlreadyExists bool
	URL           string
	FilePath      string
	Success       bool
	Error         error
}

type FFmpegEvent struct {
	Percent float64
}
