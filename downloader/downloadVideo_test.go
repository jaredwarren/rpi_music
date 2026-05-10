package downloader

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDownloadOutputPath(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{
			name: "merger output",
			out:  `[Merger] Merging formats into "/downloads/Test-abc.webm"` + "\n",
			want: "/downloads/Test-abc.webm",
		},
		{
			name: "download destination output",
			out:  `[download] Destination: /downloads/Test-abc.webm` + "\n",
			want: "/downloads/Test-abc.webm",
		},
		{
			name: "extract audio destination output",
			out:  `[ExtractAudio] Destination: /downloads/Test-abc.opus` + "\n",
			want: "/downloads/Test-abc.opus",
		},
		{
			name: "takes last match",
			out: `[download] Destination: /downloads/old.webm` + "\n" +
				`[download] Destination: /downloads/new.webm` + "\n",
			want: "/downloads/new.webm",
		},
		{
			name: "no match",
			out:  `some unrelated output`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseDownloadOutputPath(tt.out))
		})
	}
}
