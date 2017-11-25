package gbfs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	pb_dur "github.com/golang/protobuf/ptypes/duration"
	pb_ts "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/skidder/gbfs-go/gbfs/proto"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type Client struct {
	feedURL string
}

// NewClient creates a client capable of retrieve GBFS Feed data.
func NewClient(feedURL string) *Client {
	return &Client{
		feedURL: feedURL,
	}
}

func (g *Client) GetAutoDiscoveryMessage() (*proto.AutoDiscoveryMessage, error) {
	target := &proto.AutoDiscoveryMessage{}
	err := download(g.feedURL, target)
	if err != nil {
		return nil, err
	}
	target.LastUpdatedTimestamp = &pb_ts.Timestamp{Seconds: int64(target.LastUpdated)}
	target.TtlDuration = &pb_dur.Duration{Seconds: int64(target.Ttl)}
	return target, nil
}

func (g *Client) GetStationStatus(language string) (*proto.StationStatusMessage, error) {
	feedURL, err := g.getChildFeedURL(language, "station_status")
	if err != nil {
		return nil, err
	}

	target := &proto.StationStatusMessage{}
	err = download(feedURL, target)
	if err != nil {
		return nil, err
	}
	target.LastUpdatedTimestamp = &pb_ts.Timestamp{Seconds: int64(target.LastUpdated)}
	target.TtlDuration = &pb_dur.Duration{Seconds: int64(target.Ttl)}
	for _, station := range target.Data.Stations {
		station.LastReportedTimestamp = &pb_ts.Timestamp{Seconds: int64(station.LastReported)}
	}
	return target, nil
}

func (g *Client) GetStationInformation(language string) (*proto.StationInformationMessage, error) {
	feedURL, err := g.getChildFeedURL(language, "station_information")
	if err != nil {
		return nil, err
	}

	target := &proto.StationInformationMessage{}
	err = download(feedURL, target)
	if err != nil {
		return nil, err
	}
	target.LastUpdatedTimestamp = &pb_ts.Timestamp{Seconds: int64(target.LastUpdated)}
	target.TtlDuration = &pb_dur.Duration{Seconds: int64(target.Ttl)}
	return target, nil
}

// Locates the child-feed URL from the auto-discovery data using the supplied language.
func (g *Client) getChildFeedURL(language string, feedName string) (string, error) {
	autoDiscoveryMessage, err := g.GetAutoDiscoveryMessage()
	if err != nil {
		return "", err
	}
	languageEntry, found := autoDiscoveryMessage.Data[language]
	if !found {
		return "", fmt.Errorf("Language not found in auto-discovery response: %s", language)
	}
	var feedURL string
	for _, feed := range languageEntry.Feeds {
		if feedName == feed.Name {
			feedURL = feed.Url
		}
	}
	if feedURL == "" {
		return "", fmt.Errorf("Feed URL not found for feed name: %s", feedName)
	}
	return feedURL, nil
}

func download(url string, target interface{}) error {
	r, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	err = json.NewDecoder(r.Body).Decode(target)
	if err != nil {
		return err
	}
	return nil
}
