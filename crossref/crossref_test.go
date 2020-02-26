package crossref

import (
	"os"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	type args struct {
		apiBase string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name:    "A",
			args:    args{apiBase: "http://api.crossref.org/v1"},
			want:    []string{"http://api.crossref.org/v1/", "http://api.crossref.org/v1/works"},
			wantErr: false,
		},
		{
			name:    "B",
			args:    args{apiBase: "http://api.crossref.org/v1/"},
			want:    []string{"http://api.crossref.org/v1/", "http://api.crossref.org/v1/works"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.apiBase)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.APIBase().String(), tt.want[0]) {
				t.Errorf("New() = %v, want %v", got, tt.want[0])
			}
			if !reflect.DeepEqual(got.WorksPath().String(), tt.want[1]) {
				t.Errorf("New() = %v, want %v", got, tt.want[1])
			}
		})
	}
}

func TestGetWork(t *testing.T) {
	c, err := New("http://api.crossref.org/v1")
	if err != nil {
		t.Fatal(err)
	}

	ids := []string{
		"10.3390/act7010007",
		"10.1109/JSEN.2018.2797526",
	}

	for _, v := range ids {
		id := DOI(v)
		work, err := GetWork(c, id)
		if err != nil {
			t.Logf("%+v", work)
			t.Fatal(err)
		}
	}
}

func Test_decodeWork(t *testing.T) {
	args := []string{
		"testdata/work1.json",
		"testdata/work2.json",
	}

	for _, arg := range args {
		f, err := os.Open(arg)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		work, err := decodeWork(f)
		if err != nil {
			t.Fatal(err)
		}
		if len(work.Authors) == 0 {
			t.Fatal("really want authors here")
		}
	}
}

func TestDOIFromURL(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    DOI
		wantErr bool
	}{
		{
			name:    "A",
			args:    args{s: "https://doi.org/10.3390/act7010007"},
			want:    DOI("10.3390/act7010007"),
			wantErr: false,
		},
		{
			name:    "B",
			args:    args{s: "https://doi.org/10.3390/act7010007/"},
			want:    DOI("10.3390/act7010007"),
			wantErr: false,
		},
		{
			name:    "C",
			args:    args{s: "doi.org/10.3390/act7010007/"},
			want:    DOI("10.3390/act7010007"),
			wantErr: false,
		},
		{
			name:    "D",
			args:    args{s: "10.1000/123456"},
			want:    DOI("10.1000/123456"),
			wantErr: false,
		},
		{
			name:    "E",
			args:    args{s: "10.1038/issn.1476-4687"},
			want:    DOI("10.1038/issn.1476-4687"),
			wantErr: false,
		},
		{
			name:    "F",
			args:    args{s: "978-12345-99990"},
			want:    DOI(""),
			wantErr: true,
		},
		{
			name:    "G",
			args:    args{s: "10.978.86123/45678"},
			want:    DOI("10.978.86123/45678"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DOIFromURL(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("DOIFromURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DOIFromURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
