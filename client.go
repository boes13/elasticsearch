package elasticsearch

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// Searcher set the contract to manage indices, synchronize data and request
type Client interface {
	// SetHttpTimeout sets timeout to use in http request in milliseconds
	SetHttpTimeout(milliseconds int)

	// CreateIndex instantiates an index
	// https://www.elasticsearch.org/guide/en/elasticsearch/reference/current/indices-create-index.html
	CreateIndex(indexName, mapping string) (*Response, error)

	// DeleteIndex deletes an existing index.
	// https://www.elasticsearch.org/guide/en/elasticsearch/reference/current/indices-delete-index.html
	DeleteIndex(indexName string) (*Response, error)

	// UpdateIndexSetting changes specific index level settings in real time
	// https://www.elasticsearch.org/guide/en/elasticsearch/reference/current/indices-update-settings.html
	UpdateIndexSetting(indexName, mapping string) (*Response, error)

	// IndexSettings allows to retrieve settings of index
	// https://www.elasticsearch.org/guide/en/elasticsearch/reference/current/indices-get-settings.html
	IndexSettings(indexName string) (Settings, error)

	// IndexExists allows to check if the index exists or not.
	// https://www.elasticsearch.org/guide/en/elasticsearch/reference/current/indices-exists.html
	IndexExists(indexName string) (bool, error)

	// Status allows to get a comprehensive status information
	Status(indices string) (*Settings, error)

	// InsertDocument adds or updates a typed JSON document in a specific index, making it searchable
	// https://www.elasticsearch.org/guide/en/elasticsearch/reference/current/docs-index_.html
	InsertDocument(indexName, documentType, identifier string, data []byte) (*InsertDocument, error)

	// Document gets a typed JSON document from the index based on its id
	// https://www.elasticsearch.org/guide/en/elasticsearch/reference/current/docs-get.html
	Document(indexName, documentType, identifier string) (*Document, error)

	// DeleteDocument deletes a typed JSON document from a specific index based on its id
	// https://www.elasticsearch.org/guide/en/elasticsearch/reference/current/docs-delete.html
	DeleteDocument(indexName, documentType, identifier string) (*Document, error)

	// Bulk makes it possible to perform many index/delete operations in a single API call.
	// This can greatly increase the indexing speed.
	// https://www.elasticsearch.org/guide/en/elasticsearch/reference/current/docs-bulk.html
	Bulk(data []byte) (*Bulk, error)

	// Search allows to execute a search query and get back search hits that match the query
	// http://www.elasticsearch.org/guide/en/elasticsearch/reference/current/docs-delete.html
	Search(indexName, documentType, data string, explain bool) (*SearchResult, error)

	// MSearch allows to execute a multi-search and get back result
	// http://www.elasticsearch.org/guide/en/elasticsearch/reference/current/search-multi-search.html
	MSearch(queries []MSearchQuery) (*MSearchResult, error)

	// Suggest allows basic auto-complete functionality.
	// http://www.elasticsearch.org/guide/en/elasticsearch/reference/current/search-suggesters-completion.html
	Suggest(indexName, data string) ([]byte, error)

	// GetIndicesFromAlias returns the list of indices the alias points to
	GetIndicesFromAlias(alias string) ([]string, error)

	// UpdateAlias updates the indices on which the alias point to.
	// The change is atomic.
	UpdateAlias(remove []string, add []string, alias string) (*Response, error)
}

// A SearchClient describes the client configuration to manage an ElasticSearch index.
type client struct {
	Host    url.URL
	Timeout int
}

// NewSearchClient creates and initializes a new ElasticSearch client, implements core api for Indexing and searching.
func NewClient(scheme, host, port string) Client {
	u := url.URL{
		Scheme: scheme,
		Host:   host + ":" + port,
	}
	return &client{Host: u}
}

// NewSearchClient creates and initializes a new ElasticSearch client, implements core api for Indexing and searching.
func NewClientFromUrl(rawurl string) Client {
	u, err := url.Parse(rawurl)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return &client{Host: *u}
}

// SetHttpTimeout sets timeout to use in http request in milliseconds
func (c *client) SetHttpTimeout(milliseconds int) {
	c.Timeout = milliseconds
}

func (c *client) CreateIndex(indexName, mapping string) (*Response, error) {
	url := c.Host.String() + "/" + indexName
	reader := bytes.NewBufferString(mapping)
	response, err := sendHTTPRequest("POST", url, reader)
	if err != nil {
		return &Response{}, err
	}

	esResp := &Response{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &Response{}, err
	}

	return esResp, nil
}

func (c *client) DeleteIndex(indexName string) (*Response, error) {
	url := c.Host.String() + "/" + indexName
	response, err := sendHTTPRequest("DELETE", url, nil)
	if err != nil {
		return &Response{}, err
	}

	esResp := &Response{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &Response{}, err
	}

	return esResp, nil
}

func (c *client) UpdateIndexSetting(indexName, mapping string) (*Response, error) {
	url := c.Host.String() + "/" + indexName + "/_settings"
	reader := bytes.NewBufferString(mapping)
	response, err := sendHTTPRequest("PUT", url, reader)
	if err != nil {
		return &Response{}, err
	}

	esResp := &Response{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &Response{}, err
	}

	return esResp, nil
}

func (c *client) IndexSettings(indexName string) (Settings, error) {
	url := c.Host.String() + "/" + indexName + "/_settings"
	response, err := sendHTTPRequest("GET", url, nil)
	if err != nil {
		return Settings{}, err
	}

	type settingsArray map[string]Settings
	dec := json.NewDecoder(bytes.NewBuffer(response))
	var info settingsArray
	err = dec.Decode(&info)
	if err != nil {
		return Settings{}, err
	}

	return info[indexName], nil
}

func (c *client) IndexExists(indexName string) (bool, error) {
	url := c.Host.String() + "/" + indexName
	httpClient := &http.Client{}
	newReq, err := httpClient.Head(url)
	if err != nil {
		return false, err
	}

	return newReq.StatusCode == http.StatusOK, nil
}

func (c *client) Status(indices string) (*Settings, error) {
	url := c.Host.String() + "/" + indices + "/_status"
	response, err := sendHTTPRequest("GET", url, nil)
	if err != nil {
		return &Settings{}, err
	}

	esResp := &Settings{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &Settings{}, err
	}

	return esResp, nil
}

func (c *client) InsertDocument(indexName, documentType, identifier string, data []byte) (*InsertDocument, error) {
	url := c.Host.String() + "/" + indexName + "/" + documentType + "/" + identifier
	reader := bytes.NewBuffer(data)
	response, err := sendHTTPRequest("POST", url, reader)
	if err != nil {
		return &InsertDocument{}, err
	}

	esResp := &InsertDocument{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &InsertDocument{}, err
	}

	return esResp, nil
}

func (c *client) Document(indexName, documentType, identifier string) (*Document, error) {
	url := c.Host.String() + "/" + indexName + "/" + documentType + "/" + identifier
	response, err := sendHTTPRequest("GET", url, nil)
	if err != nil {
		return &Document{}, err
	}

	esResp := &Document{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &Document{}, err
	}

	return esResp, nil
}

func (c *client) DeleteDocument(indexName, documentType, identifier string) (*Document, error) {
	url := c.Host.String() + "/" + indexName + "/" + documentType + "/" + identifier
	response, err := sendHTTPRequest("DELETE", url, nil)
	if err != nil {
		return &Document{}, err
	}

	esResp := &Document{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &Document{}, err
	}

	return esResp, nil
}

func (c *client) Bulk(data []byte) (*Bulk, error) {
	url := c.Host.String() + "/_bulk"
	reader := bytes.NewBuffer(data)
	response, err := sendHTTPRequest("POST", url, reader)
	if err != nil {
		return &Bulk{}, err
	}

	esResp := &Bulk{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &Bulk{}, err
	}

	return esResp, nil
}

func (c *client) Search(indexName, documentType, data string, explain bool) (*SearchResult, error) {
	if len(documentType) > 0 {
		documentType = documentType + "/"
	}

	url := c.Host.String() + "/" + indexName + "/" + documentType + "/_search"
	if explain {
		url += "?explain"
	}
	reader := bytes.NewBufferString(data)
	response, err := sendHTTPRequest("POST", url, reader)
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
	response, err := sendHTTPRequest("POST", url, reader)

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
	response, err := sendHTTPRequest("POST", url, reader)
	return response, err
}

func (c *client) GetIndicesFromAlias(alias string) ([]string, error) {
	url := c.Host.String() + "/*/_alias/" + alias
	response, err := sendHTTPRequest("GET", url, nil)
	if err != nil {
		return []string{}, err
	}

	esResp := make(map[string]*json.RawMessage)
	err = json.Unmarshal(response, &esResp)
	if err != nil {
		return []string{}, err
	}

	indices := make([]string, len(esResp))
	i := 0
	for k := range esResp {
		indices[i] = k
		i++
	}
	return indices, nil
}

func (c *client) UpdateAlias(remove []string, add []string, alias string) (*Response, error) {
	url := c.Host.String() + "/_aliases"
	body := getAliasQuery(remove, add, alias)
	reader := bytes.NewBufferString(body)

	response, err := sendHTTPRequest("POST", url, reader)
	if err != nil {
		return &Response{}, err
	}

	esResp := &Response{}
	err = json.Unmarshal(response, esResp)
	if err != nil {
		return &Response{}, err
	}

	return esResp, nil
}

func getAliasQuery(remove []string, add []string, alias string) string {
	actions := make([]string, len(remove)+len(add))

	i := 0
	for _, index := range remove {
		actions[i] = "{ \"remove\": { \"index\": \"" + index + "\", \"alias\": \"" + alias + "\" }}"
		i++
	}

	for _, index := range add {
		actions[i] = "{ \"add\": { \"index\": \"" + index + "\", \"alias\": \"" + alias + "\" }}"
		i++
	}

	return "{\"actions\": [ " + strings.Join(actions, ",") + " ]}"
}

func sendHTTPRequest(method, url string, body io.Reader) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	newReq, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer newReq.Body.Close()
	response, err := ioutil.ReadAll(newReq.Body)
	if err != nil {
		return nil, err
	}

	if newReq.StatusCode > http.StatusCreated && newReq.StatusCode < http.StatusNotFound {
		return nil, errors.New(string(response))
	}

	return response, nil
}
