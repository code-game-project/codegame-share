package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/code-game-project/go-utils/external"
	"github.com/code-game-project/go-utils/server"
	"github.com/go-chi/chi/v5"
)

func (s *Server) registerRoutes() {
	s.Router.Post("/game", s.handleGame)
	s.Router.Post("/spectate", s.handleSpectate)
	s.Router.Post("/session", s.handleSession)
	s.Router.Get("/{id}", s.handleGet)
}

type idObj struct {
	Id string `json:"id"`
}

type gameObj struct {
	GameURL string `json:"game_url" validate:"required"`
	GameId  string `json:"game_id" validate:"required,uuid"`
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
		var game gameObj
		json.Unmarshal(obj.Data, &game)
		err = game.validate()
		if err != nil {
			respondError(w, http.StatusOK, err.Error())
			return
		}
		// TODO: show frontend with copyable game ID and some other information
		respond(w, http.StatusOK, game)
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
		http.Redirect(w, r, fmt.Sprintf("%s/spectate?game_id=%s&player_id=%s&player_secret=%s", spectate.GameURL, spectate.GameId, spectate.PlayerId, spectate.PlayerSecret), http.StatusTemporaryRedirect)
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

func (g gameObj) validate() error {
	if !isValidGameURL(g.GameURL) {
		return fmt.Errorf("'%s' is not a CodeGame game server!", g.GameURL)
	}

	if !gameExists(g.GameURL, g.GameId) {
		return fmt.Errorf("The game '%s' does not exist!", g.GameId)
	}

	return nil
}

func (g spectateObj) validate() error {
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
