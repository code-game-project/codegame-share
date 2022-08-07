package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/code-game-project/go-utils/external"
	"github.com/code-game-project/go-utils/server"
	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/didip/tollbooth_chi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

//go:embed assets
var assets embed.FS

//go:embed templates/game.tmpl
var gameTemplate string

func (s *Server) registerRoutes() {
	limiter := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Hour,
	}).SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP", "RemoteAddr"}).SetMethods([]string{"GET", "POST"})
	s.Router.Use(tollbooth_chi.LimitHandler(limiter))

	s.Router.Use(cors.AllowAll().Handler)

	s.Router.Post("/game", s.handleGame)
	s.Router.Post("/spectate", s.handleSpectate)
	s.Router.Post("/session", s.handleSession)
	s.Router.Get("/{id}", s.handleGet)

	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".js", "text/javascript")

	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	s.Router.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.FS(sub))))

	s.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/code-game-project/codegame-share/blob/main/README.md", http.StatusTemporaryRedirect)
	})
}

type idObj struct {
	Id string `json:"id"`
}

type gameObj struct {
	GameURL    string `json:"game_url" validate:"required"`
	GameId     string `json:"game_id" validate:"required,uuid"`
	JoinSecret string `json:"join_secret,omitempty"`
}

func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	var body gameObj
	err := decodeRequestBody(r, &body)
	if err != nil {
		respondDecodeError(w, err)
		return
	}
	body.GameURL = external.TrimURL(body.GameURL)

	if err = body.validate(); err != nil {
		respondError(w, http.StatusForbidden, err.Error())
		return
	}

	id, err := s.storeEntry(TypeGame, body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}
	respond(w, http.StatusCreated, idObj{
		Id: id,
	})
}

type spectateObj struct {
	GameURL      string `json:"game_url" validate:"required"`
	GameId       string `json:"game_id" validate:"required,uuid"`
	PlayerId     string `json:"player_id" validate:"required,uuid"`
	PlayerSecret string `json:"player_secret" validate:"required"`
}

func (s *Server) handleSpectate(w http.ResponseWriter, r *http.Request) {
	var body spectateObj
	err := decodeRequestBody(r, &body)
	if err != nil {
		respondDecodeError(w, err)
		return
	}
	body.GameURL = external.TrimURL(body.GameURL)

	if err = body.validate(); err != nil {
		respondError(w, http.StatusForbidden, err.Error())
		return
	}

	id, err := s.storeEntry(TypeSpectate, body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}
	respond(w, http.StatusCreated, idObj{
		Id: id,
	})
}

type sessionObj struct {
	GameURL  string `json:"game_url" validate:"required"`
	Username string `json:"username" validate:"required"`
	Session  struct {
		GameId       string `json:"game_id" validate:"required,uuid"`
		PlayerId     string `json:"player_id" validate:"required,uuid"`
		PlayerSecret string `json:"player_secret" validate:"required"`
	} `json:"session" validate:"required"`
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	var body sessionObj
	err := decodeRequestBody(r, &body)
	if err != nil {
		respondDecodeError(w, err)
		return
	}
	body.GameURL = external.TrimURL(body.GameURL)

	if err = body.validate(); err != nil {
		respondError(w, http.StatusForbidden, err.Error())
		return
	}

	id, err := s.storeEntry(TypeSession, body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}
	respond(w, http.StatusCreated, idObj{
		Id: id,
	})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	obj, err := s.getEntry(id)
	if err != nil {
		if err == ErrNotFound {
			respondError(w, http.StatusNotFound, fmt.Sprintf("No entry stored at %s.", id))
		} else {
			respondError(w, http.StatusInternalServerError, "An unexpected error occured.")
		}
		return
	}

	if entryTypeQuery := r.URL.Query().Get("type"); entryTypeQuery != "" {
		var entryType EntryType
		switch entryTypeQuery {
		case "game":
			entryType = TypeGame
		case "spectate":
			entryType = TypeSpectate
		case "session":
			entryType = TypeSession
		default:
			respondError(w, http.StatusBadRequest, "Unknown entry type: "+entryTypeQuery)
			return
		}
		if obj.Type != entryType {
			respondError(w, http.StatusNotFound, fmt.Sprintf("No entry of type '%s' stored at %s.", entryTypeQuery, id))
			return
		}
	}

	if obj.Type == TypeGame {
		getGame(obj, w, r)
		return
	}
	if obj.Type == TypeSpectate {
		var spectate spectateObj
		json.Unmarshal(obj.Data, &spectate)
		err = spectate.validate()
		if err != nil {
			respondError(w, http.StatusOK, err.Error())
			return
		}
		tls := false
		if !isLocalIP(spectate.GameURL) {
			tls = external.IsTLS(spectate.GameURL)
		}
		http.Redirect(w, r, fmt.Sprintf("%s/spectate?game_id=%s&player_id=%s&player_secret=%s", external.BaseURL("http", tls, spectate.GameURL), spectate.GameId, spectate.PlayerId, spectate.PlayerSecret), http.StatusTemporaryRedirect)
		return
	}
	if obj.Type == TypeSession {
		var session sessionObj
		json.Unmarshal(obj.Data, &session)
		err = session.validate()
		if err != nil {
			respondError(w, http.StatusOK, err.Error())
			return
		}
		respond(w, http.StatusOK, session)
		return
	}
}

func getGame(obj entry, w http.ResponseWriter, r *http.Request) {
	var game gameObj
	json.Unmarshal(obj.Data, &game)
	err := game.validate()
	if err != nil {
		respondError(w, http.StatusOK, err.Error())
		return
	}

	tmpl, err := template.New("game.html").Parse(gameTemplate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type gameTmplData struct {
		DisplayName   string
		Description   string
		BaseURL       string
		GameID        string
		JoinSecret    string
		URL           string
		PlayerCount   int
		Version       string
		RepositoryURL string
		CGVersion     string
	}

	if isLocalIP(game.GameURL) {
		tmpl.Execute(w, gameTmplData{
			DisplayName: game.GameURL,
			BaseURL:     external.BaseURL("http", false, game.GameURL),
			GameID:      game.GameId,
			JoinSecret:  game.JoinSecret,
			URL:         game.GameURL,
			PlayerCount: -1,
		})
		return
	}

	api, err := server.NewAPI(game.GameURL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	info, err := api.FetchGameInfo()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if info.DisplayName == "" {
		info.DisplayName = info.Name
	}

	players, err := api.GetPlayers(game.GameId)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	tmpl.Execute(w, gameTmplData{
		DisplayName:   info.DisplayName,
		Description:   info.Description,
		BaseURL:       external.BaseURL("http", external.IsTLS(game.GameURL), game.GameURL),
		GameID:        game.GameId,
		JoinSecret:    game.JoinSecret,
		URL:           game.GameURL,
		PlayerCount:   len(players),
		Version:       info.Version,
		RepositoryURL: info.RepositoryURL,
		CGVersion:     info.CGVersion,
	})
}

func (g gameObj) validate() error {
	if isLocalIP(g.GameURL) {
		return nil
	}

	if !isValidGameURL(g.GameURL) {
		return fmt.Errorf("'%s' is not a CodeGame game server!", g.GameURL)
	}

	if !gameExists(g.GameURL, g.GameId) {
		return fmt.Errorf("The game '%s' does not exist!", g.GameId)
	}

	return nil
}

func (g spectateObj) validate() error {
	if isLocalIP(g.GameURL) {
		return nil
	}

	if !isValidGameURL(g.GameURL) {
		return fmt.Errorf("'%s' is not a CodeGame game server!", g.GameURL)
	}

	if !gameExists(g.GameURL, g.GameId) {
		return fmt.Errorf("The game '%s' does not exist!", g.GameId)
	}

	if !playerExists(g.GameURL, g.GameId, g.PlayerId) {
		return fmt.Errorf("The player '%s' does not exist!", g.PlayerId)
	}

	return nil
}

func (g sessionObj) validate() error {
	if isLocalIP(g.GameURL) {
		return nil
	}

	if !isValidGameURL(g.GameURL) {
		return fmt.Errorf("'%s' is not a CodeGame game server!", g.GameURL)
	}

	if !gameExists(g.GameURL, g.Session.GameId) {
		return fmt.Errorf("The game '%s' does not exist!", g.Session.GameId)
	}

	if !playerExists(g.GameURL, g.Session.GameId, g.Session.PlayerId) {
		return fmt.Errorf("The player '%s' does not exist!", g.Session.PlayerId)
	}

	return nil
}

func isValidGameURL(url string) bool {
	api, err := server.NewAPI(url)
	if err != nil {
		return false
	}
	info, err := api.FetchGameInfo()
	if err != nil || info.CGVersion == "" {
		return false
	}
	return true
}

var localIpRegex = regexp.MustCompile(`(192\.168\.[0-9][0-9]?[0-9]?\.[0-9][0-9]?[0-9]?)|(10\.[0-9][0-9]?[0-9]?\.[0-9][0-9]?[0-9]?\.[0-9][0-9]?[0-9]?)`)

func isLocalIP(url string) bool {
	if strings.HasPrefix(url, "192.168.") || strings.HasPrefix(url, "10.") {
		return localIpRegex.MatchString(url)
	} else if strings.HasPrefix(url, "172.") {
		parts := strings.Split(url, ".")
		if len(parts) != 4 {
			return false
		}
		second, err := strconv.Atoi(parts[1])
		if err != nil {
			return false
		}
		return second >= 16 && second <= 31
	}
	return false
}

func gameExists(gameURL, gameId string) bool {
	baseURL := external.BaseURL("http", external.IsTLS(gameURL), gameURL)
	resp, err := http.Get(fmt.Sprintf("%s/api/games/%s/players", baseURL, gameId))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return true
}

func playerExists(gameURL, gameId, playerId string) bool {
	baseURL := external.BaseURL("http", external.IsTLS(gameURL), gameURL)
	resp, err := http.Get(fmt.Sprintf("%s/api/games/%s/players/%s", baseURL, gameId, playerId))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return true
}
