package spotify

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"spotseek/src/config"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type SpotifyClient struct {
	rdb *redis.Client
}

type TokenHttpResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type UserPlaylist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewSpotifyClient(rdb *redis.Client) *SpotifyClient {
	return &SpotifyClient{rdb: rdb}
}

func (s *SpotifyClient) LoginHandler(w http.ResponseWriter, r *http.Request) {
	scope := "user-read-currently-playing user-read-playback-state user-read-recently-played user-read-playback-position"
	state := s.generateStateCookie(w)

	queryString := url.Values{}
	queryString.Add("response_type", "code")
	queryString.Add("client_id", config.CLIENT_ID)
	queryString.Add("scope", scope)
	queryString.Add("redirect_uri", config.REDIRECT_URI)
	queryString.Add("state", state)
	http.Redirect(w, r, "https://accounts.spotify.com/authorize?"+queryString.Encode(), http.StatusTemporaryRedirect)
}

func (s *SpotifyClient) generateStateCookie(w http.ResponseWriter) string {
	expiration := time.Now().Add(365 * 24 * time.Hour)
	state := s.generateRandomString(16)
	cookie := http.Cookie{Name: "state_key", Value: state, Expires: expiration}
	http.SetCookie(w, &cookie)
	return state
}
func (s *SpotifyClient) generateRandomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		j := rand.Intn(len(letters))
		b[i] = letters[j]
	}
	return string(b)
}

func (s *SpotifyClient) CallbackTokenHandler(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	qState := r.FormValue("state")
	localState, err := r.Cookie("state_key")
	if err != nil {
		log.Fatal("Cannot read cookie")
	}
	if qState != localState.Value {
		http.Redirect(w, r, "/#?error=state_mismatch", http.StatusTemporaryRedirect)
	}
	data := url.Values{
		"code":         {code},
		"redirect_uri": {config.REDIRECT_URI},
		"grant_type":   {"authorization_code"},
	}

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data.Encode()))
	secretBuf := []byte(config.CLIENT_ID + ":" + config.CLIENT_SECRET)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString(secretBuf))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("Request to Spotify for auth token failed")
	}
	defer res.Body.Close()
	var tokenRes TokenHttpResponse
	err = json.NewDecoder(res.Body).Decode(&tokenRes)
	if err != nil {
		log.Fatal("Cannot read Spotify user content")
	}
	jsonToSend, err := json.Marshal(tokenRes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonToSend)
}
