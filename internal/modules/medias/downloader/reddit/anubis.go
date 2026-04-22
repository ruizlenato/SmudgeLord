package reddit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
)

func looksLikeBlockPage(htmlContent string) bool {
	return strings.Contains(htmlContent, "anubis_challenge") ||
		strings.Contains(strings.ToLower(htmlContent), "making sure you're not a bot") ||
		strings.Contains(htmlContent, "<title>403")
}

type anubisChallengeInfo struct {
	Algorithm   string
	Difficulty  int
	ChallengeID string
	RandomData  string
	PassURL     string
	Redir       string
}

func extractAnubisChallengeInfo(htmlContent string, challengeURL string) *anubisChallengeInfo {
	re := regexp.MustCompile(`(?is)<script[^>]*\bid=["']anubis_challenge["'][^>]*>(.*?)</script>`)
	match := re.FindStringSubmatch(htmlContent)
	if len(match) < 2 {
		return nil
	}

	payloadStr := html.UnescapeString(strings.TrimSpace(match[1]))
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return nil
	}

	rules, ok := payload["rules"].(map[string]interface{})
	if !ok {
		return nil
	}
	challenge, ok := payload["challenge"].(map[string]interface{})
	if !ok {
		return nil
	}

	algorithm := ""
	if v, ok := rules["algorithm"].(string); ok {
		algorithm = v
	}
	if algorithm == "" {
		if v, ok := challenge["method"].(string); ok {
			algorithm = v
		}
	}
	algorithm = strings.ToLower(strings.TrimSpace(algorithm))

	difficulty := 0
	if v, ok := rules["difficulty"].(float64); ok {
		difficulty = int(v)
	}
	if difficulty == 0 {
		if v, ok := challenge["difficulty"].(float64); ok {
			difficulty = int(v)
		}
	}

	challengeID := ""
	if v, ok := challenge["id"].(string); ok {
		challengeID = strings.TrimSpace(v)
	}

	randomData := ""
	if v, ok := challenge["randomData"].(string); ok {
		randomData = strings.TrimSpace(v)
	}

	if algorithm == "" || challengeID == "" || randomData == "" {
		return nil
	}

	basePrefix := ""
	rePref := regexp.MustCompile(`(?is)<script[^>]*\bid=["']anubis_base_prefix["'][^>]*>(.*?)</script>`)
	if m := rePref.FindStringSubmatch(htmlContent); len(m) >= 2 {
		basePrefix = strings.TrimSpace(html.UnescapeString(strings.Trim(m[1], "\"' ")))
	}

	prefix := basePrefix
	if prefix != "" && !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	prefix = strings.TrimRight(prefix, "/")

	u, _ := url.Parse(challengeURL)
	baseURL := ""
	if u != nil {
		baseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	passURL := fmt.Sprintf("%s%s/.within.website/x/cmd/anubis/api/pass-challenge", baseURL, prefix)

	redir := "/"
	if u != nil {
		redir = u.Path
		if u.RawQuery != "" {
			redir += "?" + u.RawQuery
		}
	}

	return &anubisChallengeInfo{
		Algorithm:   algorithm,
		Difficulty:  difficulty,
		ChallengeID: challengeID,
		RandomData:  randomData,
		PassURL:     passURL,
		Redir:       redir,
	}
}

func solvePowChallenge(randomData string, difficulty int) (string, int, error) {
	targetPrefix := strings.Repeat("0", difficulty)
	nonce := 0

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		data := fmt.Sprintf("%s%d", randomData, nonce)
		hash := sha256.Sum256([]byte(data))
		hashStr := hex.EncodeToString(hash[:])

		if strings.HasPrefix(hashStr, targetPrefix) {
			return hashStr, nonce, nil
		}
		nonce++
	}

	return "", 0, fmt.Errorf("PoW challenge not solved within timeout")
}

func solveAnubisChallengeForURL(challengeURL string, client *http.Client) (*http.Response, error) {
	jar, _ := cookiejar.New(nil)
	client.Jar = jar

	req, err := http.NewRequest("GET", challengeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range downloader.GenericHeaders {
		req.Header.Set(k, v)
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge page: %w", err)
	}

	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read challenge page: %w", err)
	}
	htmlContent := string(body)

	if !looksLikeBlockPage(htmlContent) {
		resp := &http.Response{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Header:     response.Header,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}
		return resp, nil
	}

	info := extractAnubisChallengeInfo(htmlContent, challengeURL)
	if info == nil {
		return nil, fmt.Errorf("failed to extract anubis challenge info")
	}

	if info.Algorithm != "fast" && info.Algorithm != "slow" {
		return nil, fmt.Errorf("unsupported algorithm: %s", info.Algorithm)
	}

	responseHash, nonce, err := solvePowChallenge(info.RandomData, info.Difficulty)
	if err != nil {
		return nil, fmt.Errorf("failed to solve PoW: %w", err)
	}

	elapsedMs := 100

	passURL := fmt.Sprintf("%s?id=%s&response=%s&nonce=%d&redir=%s&elapsedTime=%d",
		info.PassURL,
		url.QueryEscape(info.ChallengeID),
		url.QueryEscape(responseHash),
		nonce,
		url.QueryEscape(info.Redir),
		elapsedMs,
	)

	passResp, err := client.Get(passURL)
	if err != nil {
		return nil, fmt.Errorf("failed to pass challenge: %w", err)
	}

	_, err = io.ReadAll(passResp.Body)
	passResp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read pass response: %w", err)
	}

	req2, err := http.NewRequest("GET", challengeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range downloader.GenericHeaders {
		req2.Header.Set(k, v)
	}

	finalResp, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("failed to get final response: %w", err)
	}

	finalBody, err := io.ReadAll(finalResp.Body)
	finalResp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read final response: %w", err)
	}

	newResp := &http.Response{
		StatusCode: finalResp.StatusCode,
		Status:     finalResp.Status,
		Header:     finalResp.Header,
		Body:       io.NopCloser(bytes.NewReader(finalBody)),
		Request:    req2,
	}
	return newResp, nil
}

func solveAnubisMediaForURL(mediaURL string, client *http.Client) (*downloader.FetchInfo, error) {
	req, err := http.NewRequest("GET", mediaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range downloader.GenericHeaders {
		req.Header.Set(k, v)
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge page: %w", err)
	}

	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read challenge page: %w", err)
	}
	htmlContent := string(body)

	if !looksLikeBlockPage(htmlContent) {
		return &downloader.FetchInfo{
			Body:        body,
			StatusCode:  response.StatusCode,
			ContentType: response.Header.Get("Content-Type"),
		}, nil
	}

	info := extractAnubisChallengeInfo(htmlContent, mediaURL)
	if info == nil {
		return nil, fmt.Errorf("failed to extract anubis challenge info")
	}

	if info.Algorithm != "fast" && info.Algorithm != "slow" {
		return nil, fmt.Errorf("unsupported algorithm: %s", info.Algorithm)
	}

	responseHash, nonce, err := solvePowChallenge(info.RandomData, info.Difficulty)
	if err != nil {
		return nil, fmt.Errorf("failed to solve PoW: %w", err)
	}

	elapsedMs := 100

	passURL := fmt.Sprintf("%s?id=%s&response=%s&nonce=%d&redir=%s&elapsedTime=%d",
		info.PassURL,
		url.QueryEscape(info.ChallengeID),
		url.QueryEscape(responseHash),
		nonce,
		url.QueryEscape(info.Redir),
		elapsedMs,
	)

	passResp, err := client.Get(passURL)
	if err != nil {
		return nil, fmt.Errorf("failed to pass challenge: %w", err)
	}

	_, err = io.ReadAll(passResp.Body)
	passResp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read pass response: %w", err)
	}

	req2, err := http.NewRequest("GET", mediaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range downloader.GenericHeaders {
		req2.Header.Set(k, v)
	}

	finalResp, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("failed to get final response: %w", err)
	}

	finalBody, err := io.ReadAll(finalResp.Body)
	finalResp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read final response: %w", err)
	}

	return &downloader.FetchInfo{
		Body:        finalBody,
		StatusCode:  finalResp.StatusCode,
		ContentType: finalResp.Header.Get("Content-Type"),
	}, nil
}
