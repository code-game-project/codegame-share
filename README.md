# codegame-share

A service to make sharing game IDs, spectate links and sessions easier.

## API

### POST /game

#### Description

Stores the game URL and ID and returns an 8 characters long ID
which when presented to the `/{id}` endpoint will return a web page with information about the game.

#### Request Body

```jsonc
{
	"game_url": "", // URL of the game server (e.g. games.code-game.org/hoverrace)
	"game_id": "" // ID of the game
}
```

### POST /spectate

#### Description

Stores the game URL, game ID, player ID and player secret and returns an 8 characters long ID,
which when presented to the `/{id}` endpoint will redirect the user to the respective spectate view of the game server.

#### Request Body

```jsonc
{
	"game_url": "", // URL of the game server (e.g. games.code-game.org/hoverrace)
	"game_id": "", // ID of the game
	"player_id": "", // ID of the player
	"player_secret": "" // the player secret
}
```

### POST /session

#### Description

Stores a session object and returns an 8 characters long ID,
which when presented to the `/{id}` endpoint will return the session again.

#### Request Body

```jsonc
{
	"game_url": "", // URL of the game server (e.g. games.code-game.org/hoverrace)
	"username": "", // the username of the player
	"session": {
		"game_id": "", // ID of the game
		"player_id": "", // ID of the player
		"player_secret": "" // the player secret
	}
}
```

### GET /{id}

Retrieves the data associated with the ID and returns it in a different format depending on the endpoint used to store it.

IDs and their associated content are stored for 24h.

## License

Copyright (c) 2022 Julian Hofmann

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
