package mongodb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Client struct {
	Mongo    *mongo.Client
	Database *mongo.Database
}

type tcpDialer struct {
	dialer *net.Dialer
}

func (d *tcpDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dialer.DialContext(ctx, "tcp4", address)
}

func directAtlasURI(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "mongodb+srv" {
		return ""
	}
	_, records, err := net.LookupSRV("mongodb", "tcp", parsed.Host)
	if err != nil {
		return ""
	}
	if len(records) == 0 {
		return ""
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Port < records[j].Port
	})
	hosts := make([]string, 0, len(records))
	for _, record := range records {
		hosts = append(hosts, net.JoinHostPort(strings.TrimSuffix(record.Target, "."), fmt.Sprint(record.Port)))
	}
	parsed.Scheme = "mongodb"
	parsed.Host = strings.Join(hosts, ",")
	query := parsed.Query()
	query.Set("tls", "true")
	query.Set("authSource", "admin")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func Connect(ctx context.Context, uri, databaseName string) (*Client, error) {
	if uri == "" {
		return nil, errors.New("MONGODB_URI is not configured")
	}

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	connectURI := uri
	opts := options.Client().
		ApplyURI(connectURI).
		SetServerSelectionTimeout(15 * time.Second).
		SetConnectTimeout(10 * time.Second).
		SetDialer(&tcpDialer{dialer: dialer})

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		directURI := directAtlasURI(uri)
		if directURI == "" {
			return nil, err
		}
		opts = options.Client().
			ApplyURI(directURI).
			SetServerSelectionTimeout(15 * time.Second).
			SetConnectTimeout(10 * time.Second).
			SetDialer(&tcpDialer{dialer: dialer})
		client, directErr := mongo.Connect(ctx, opts)
		if directErr != nil {
			return nil, directErr
		}
		if directErr := client.Ping(ctx, nil); directErr != nil {
			_ = client.Disconnect(ctx)
			return nil, directErr
		}
	}

	return &Client{
		Mongo:    client,
		Database: client.Database(databaseName),
	}, nil
}

func (c *Client) Close(ctx context.Context) error {
	if c == nil || c.Mongo == nil {
		return nil
	}
	return c.Mongo.Disconnect(ctx)
}
