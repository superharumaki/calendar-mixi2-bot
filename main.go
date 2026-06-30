package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const stateFile = "state.json"

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

var eventCodeRe = regexp.MustCompile(`\s*[0-9]{6,}$`)
var channelPrefixRe = regexp.MustCompile(`^CS\s*[^“"]+`)
var locationBracketRe = regexp.MustCompile(`^[（(](.+)[）)]$`)

var calendars = []string{
	"cafa1c6ip201o1jj80r82mqu00@group.calendar.google.com",
	"5r60kb9t5ttr22d07q13rrj590@group.calendar.google.com",
	"a.f.calendar.ver2@gmail.com",
	"b9m0meq9124s6a57ndbofua09g@group.calendar.google.com",
}

type State struct {
	LastPostDate string `json:"last_post_date"`
}

type CalendarEvents struct {
	Items []Event `json:"items"`
}

type Event struct {
	Summary     string `json:"summary"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Start       struct {
		Date     string `json:"date"`
		DateTime string `json:"dateTime"`
	} `json:"start"`
	End struct {
		Date     string `json:"date"`
		DateTime string `json:"dateTime"`
	} `json:"end"`
}

func main() {
	apiKey := requireEnv("GOOGLE_API_KEY")

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatal("タイムゾーン設定失敗:", err)
	}

	today := time.Now().In(loc).Format("2006-01-02")
	state := loadState()

	if state.LastPostDate == today {
		log.Println("今日はすでに投稿済みなので終了")
		return
	}

	now := time.Now().In(loc)
	tomorrow := now.AddDate(0, 0, 1)

	start := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)

	var posts []string

	for _, calID := range calendars {
		events, err := fetchEvents(calID, apiKey, start, end)
		if err != nil {
			log.Printf("calendar error %s: %v", calID, err)
			continue
		}

		for _, e := range events {
			post := buildPostText(e)
			if post != "" {
				posts = append(posts, post)
			}
		}
	}

	if len(posts) == 0 {
		log.Println("明日の予定はありません。投稿しません。")
		return
	}

	text := formatDate(tomorrow) + "\n\n" + strings.Join(posts, "\n\n")
	text = trimPostText(text)

	if os.Getenv("PREVIEW") == "1" {
		log.Println("プレビュー:")
		log.Println(text)
		return
	}

	if err := postToMixi2(text); err != nil {
		log.Fatal("投稿失敗:", err)
	}

	log.Println("投稿成功:", text)

	state.LastPostDate = today
	saveState(state)
}

func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatal(key + " missing value")
	}

	return value
}

func buildPostText(e Event) string {
	channelText := cleanLocation(e.Location)
	titleText := cleanTitle(e.Summary)
	subtitleText := cleanSubtitle(e.Description)

	var lines []string

	if channelText != "" {
		lines = append(lines, channelText)
	}
	if titleText != "" {
		lines = append(lines, titleText)
	}
	if subtitleText != "" {
		lines = append(lines, subtitleText)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func trimPostText(text string) string {
	// mixi2投稿本文の上限に収まるようにするための最大文字数
	const maxPostLen = 147

	runes := []rune(text)
	if len(runes) <= maxPostLen {
		return text
	}

	return string(runes[:maxPostLen-1]) + "…"
}

func fetchEvents(calendarID string, apiKey string, start time.Time, end time.Time) ([]Event, error) {
	base := "https://www.googleapis.com/calendar/v3/calendars/" + url.PathEscape(calendarID) + "/events"

	q := url.Values{}
	q.Set("key", apiKey)
	q.Set("timeMin", start.Format(time.RFC3339))
	q.Set("timeMax", end.Format(time.RFC3339))
	q.Set("singleEvents", "true")
	q.Set("orderBy", "startTime")
	q.Set("timeZone", "Asia/Tokyo")

	reqURL := base + "?" + q.Encode()

	resp, err := httpClient.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Calendar API status %d: %s", resp.StatusCode, string(body))
	}

	var data CalendarEvents
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	return data.Items, nil
}

func formatDate(t time.Time) string {
	weekdays := []string{"日", "月", "火", "水", "木", "金", "土"}

	return fmt.Sprintf(
		"%d月%d日（%s）",
		t.Month(),
		t.Day(),
		weekdays[t.Weekday()],
	)
}

func cleanLocation(location string) string {
	location = strings.TrimSpace(location)

	if m := locationBracketRe.FindStringSubmatch(location); m != nil {
		return strings.TrimSpace(m[1])
	}

	return location
}

func cleanTitle(summary string) string {
	summary = strings.TrimSpace(summary)

	summary = eventCodeRe.ReplaceAllString(summary, "")
	summary = channelPrefixRe.ReplaceAllString(summary, "")

	return strings.TrimSpace(summary)
}

func cleanSubtitle(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}

	lines := strings.Split(description, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if strings.Contains(line, "http://") || strings.Contains(line, "https://") {
			continue
		}

		return line
	}

	return ""
}

func postToMixi2(text string) error {
	authenticator, err := auth.NewAuthenticator(
		requireEnv("CLIENT_ID"),
		requireEnv("CLIENT_SECRET"),
		requireEnv("TOKEN_URL"),
	)
	if err != nil {
		return err
	}

	ctx, err := authenticator.AuthorizedContext(context.Background())
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(
		requireEnv("API_ADDRESS"),
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""),
		),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := application_apiv1.NewApplicationServiceClient(conn)

	_, err = client.CreatePost(
		ctx,
		&application_apiv1.CreatePostRequest{
			Text: text,
		},
	)

	return err
}

func loadState() State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return State{}
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}
	}

	return s
}

func saveState(s State) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Fatal("state保存失敗:", err)
	}

	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		log.Fatal("state書き込み失敗:", err)
	}
}
