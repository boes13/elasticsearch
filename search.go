package elasticsearch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"net/http"
)

func (c *client) Search(indexName, documentType, data string, explain bool) (*SearchResult, error) {
	if len(documentType) > 0 {
		documentType = documentType + "/"
	}

	url := c.Host.String() + "/" + indexName + "/" + documentType + "/_search"
	if explain {
		url += "?explain"
	}
	reader := bytes.NewBufferString(data)
	response, err := sendHTTPRequest("POST", url, reader, c.Timeout)
	if err != nil {
		return &SearchResult{}, err
	}

	esResp := &SearchResult{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &SearchResult{}, err
	}

	return esResp, nil
}

func (c *client) MSearch(queries []MSearchQuery) (*MSearchResult, error) {
	replacer := strings.NewReplacer("\n", " ")
	queriesList := make([]string, len(queries))
	for i, query := range queries {
		queriesList[i] = query.Header + "\n" + replacer.Replace(query.Body)
	}

	mSearchQuery := strings.Join(queriesList, "\n") + "\n" // Don't forget trailing \n
	url := c.Host.String() + "/_msearch"
	reader := bytes.NewBufferString(mSearchQuery)
	response, err := sendHTTPRequest("POST", url, reader, c.Timeout)

	if err != nil {
		return &MSearchResult{}, err
	}

	esResp := &MSearchResult{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &MSearchResult{}, err
	}

	return esResp, nil
}

func (c *client) Suggest(indexName, data string) ([]byte, error) {
	url := c.Host.String() + "/" + indexName + "/_suggest"
	reader := bytes.NewBufferString(data)
	response, err := sendHTTPRequest("POST", url, reader, c.Timeout)
	return response, err
}

type Scroller struct {
	ScrollId string `json:"_scroll_id"`
	Took     uint64 `json:"took"`
	TimedOut bool   `json:"timed_out"`
	Shards   struct {
		Total      int `json:"total"`
		Successful int `json:"successful"`
		Failed     int `json:"failed"`
	} `json:"_shards"`
	Hits    ResultHits `json:"hits"`
	baseUrl string
	expire  string
	httpTimeout time.Duration
}

func (c *client) SearchByScanAndScroll(indexName string, documentType string, expireTime time.Duration, body string) (*Scroller, error) {
	// parameter validation
	if indexName == "" || documentType == "" || expireTime.Nanoseconds() == 0 {
		return nil, errors.New("Either indexName, documentType, or expirationTime parameter is invalid!")
	}
	expire := fmt.Sprintf("%ds", int(expireTime.Seconds()))
	url := fmt.Sprintf("%s/%s/%s/_search?search_type=scan&scroll=%s", c.Host.String(), indexName, documentType, expire)
	reader := bytes.NewBufferString(body)
	response, err := sendHTTPRequest(http.MethodPost, url, reader, c.Timeout)
	if err != nil {
		return nil, err
	}

	scroller := &Scroller{}
	scroller.baseUrl = c.Host.String()
	scroller.expire = expire
	scroller.httpTimeout = c.Timeout
	err = json.Unmarshal(response, scroller)
	return scroller, err
}

func (scroller *Scroller) NextChunk() error {
	url := fmt.Sprintf("%s/_search/scroll?scroll=%s&scroll_id=%s", scroller.baseUrl, scroller.expire, scroller.ScrollId)
	response, err := sendHTTPRequest(http.MethodGet, url, nil, scroller.httpTimeout)
	if err != nil {
		return err
	}
	err = json.Unmarshal(response, scroller)
	return err
}
