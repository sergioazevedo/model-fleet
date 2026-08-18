package providertest

import (
	"io"
	"net/http"
	"strings"
)

type RequestSpy struct {
	ResponseBody   string
	ResponseStatus int
	Request        *http.Request
	RequestBody    string
}

func (s *RequestSpy) RoundTrip(req *http.Request) (*http.Response, error) {
	defer req.Body.Close()

	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	s.Request = req
	s.RequestBody = string(reqBody)

	return &http.Response{
		StatusCode: s.ResponseStatus,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(s.ResponseBody)),
	}, nil
}

func NewHTTPClient(responseBody string, responseStatus int) (*http.Client, *RequestSpy) {
	spy := &RequestSpy{
		ResponseBody:   responseBody,
		ResponseStatus: responseStatus,
	}

	return &http.Client{Transport: spy}, spy
}
