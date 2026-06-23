package domain

type PlaylistType int

const (
	PlaylistUnknown PlaylistType = iota
	PlaylistMaster
	PlaylistTS
	PlaylistFMP4
)

type Segment struct {
	URL      string
	Duration float64
}

type VideoVariation struct {
	URL              string
	Bandwidth        int
	AverageBandwidth int
	Resolution       string
	Codecs           []string
}

type Playlist struct {
	URL             string
	Type            PlaylistType
	VideoVariations []VideoVariation
	Segments        []Segment
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
