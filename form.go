package chichi

import (
	"net/http"
	"net/url"
)

type formDecoder struct {
	maxMemory int64
}

func (d formDecoder) Decode(r *http.Request, dst any) error {
	var values url.Values
	if mediaType(r.Header.Get("Content-Type")) == mimeMultipart {
		if err := r.ParseMultipartForm(d.maxMemory); err != nil {
			return err
		}
		values = r.MultipartForm.Value
	} else {
		if err := r.ParseForm(); err != nil {
			return err
		}
		values = r.PostForm
	}
	return decodeValues(values, dst, "form")
}
