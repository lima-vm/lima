package iso9660util

type ISO9660 struct {
	Name                  string            // "cidata"
	FilesFromContent      map[string]string // use string, not []byte, for debug printability
	FilesFromHostFilePath map[string]string
}
