package elasticsearch

import (
	"testing"
	"time"
	"encoding/json"
	"fmt"
)

func TestClient_SearchByScanAndScroll(t *testing.T) {
	client := NewClient("http", "localhost", "9200")
	body := `{
			"_source": false,
			"query": {
				"range" : {
				    "ctd_at" : {
					"gte": "2017-03-01 00:00:00",
					"lt": "2017-03-01 18:00:00"

				    }
				}
			    },
		    	"size": 10
		}`
	scroller, err := client.SearchByScanAndScroll("order", "all", 1 * time.Minute, body)
	if err != nil {
		t.Error("Expected no err, got error:", err.Error())
	}

	type test struct {
		ID int64 `json:"oid"`
		Invoice string `json:"inv_rn"`
		ShopId int64 `json:"shop_id"`
		CustomerId int64 `json:"cust_id"`
		CustomerName string `json:"cust_nm"`
		ReceiverName string `json:"rcvr_nm"`
		CreatedDate string `json:"ctd_at"`
		ShopName string `json:"shop_nm"`
	}

	sum := 0
	for {
		err = scroller.NextChunk()
		if err != nil {
			t.Error("Found err:", err.Error())
		}
		nbHits := len(scroller.Hits.Hits)
		sum += nbHits
		if nbHits > 0 {
			rawJson := scroller.Hits.Hits[0].Source
			te := test{}
			json.Unmarshal(rawJson, &te)
		} else {
			// no more data in scroll
			break
		}
	}
	fmt.Println(sum)
}
