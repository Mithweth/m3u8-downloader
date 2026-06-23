package domain

type PlaylistType int

const (
	PlaylistUnknown PlaylistType = iota
	PlaylistMaster
	PlaylistTS
	PlaylistFMP4
)

type ByteRange struct {
	Offset int64
	Length int64
}

type Segment struct {
	URL      string
	Name     string
	Duration float64
	Range    *ByteRange
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
