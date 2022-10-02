package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/jaredwarren/rpi_music/graph/generated"
	"github.com/jaredwarren/rpi_music/graph/model"
)

// CreateLink is the resolver for the createLink field.
func (r *mutationResolver) CreateLink(ctx context.Context, input model.NewLink) (*model.Link, error) {
	var link model.Link
	var user model.User
	link.Address = input.Address
	link.Title = input.Title
	user.Name = "test"
	link.User = &user
	return &link, nil
}

// CreateUser is the resolver for the createUser field.
func (r *mutationResolver) CreateUser(ctx context.Context, input model.NewUser) (string, error) {
	panic(fmt.Errorf("not implemented: CreateUser - createUser"))
}

// Login is the resolver for the login field.
func (r *mutationResolver) Login(ctx context.Context, input model.Login) (string, error) {
	panic(fmt.Errorf("not implemented: Login - login"))
}

// Logout is the resolver for the logout field.
func (r *mutationResolver) Logout(ctx context.Context, input *bool) (bool, error) {
	panic(fmt.Errorf("not implemented: Logout - logout"))
}

// RefreshToken is the resolver for the refreshToken field.
func (r *mutationResolver) RefreshToken(ctx context.Context, input model.RefreshTokenInput) (string, error) {
	panic(fmt.Errorf("not implemented: RefreshToken - refreshToken"))
}

// EditSong is the resolver for the editSong field.
func (r *mutationResolver) EditSong(ctx context.Context, input model.NewSong) (*model.Song, error) {
	// TODO: call something like: UpdateSongHandler
	// but change logic to that no id = new
	id := "new"
	if input.ID != nil {
		id = *input.ID
	}
	return &model.Song{
		ID:  id,
		URL: input.URL,
		// RFID: input.Rfid,
	}, nil
}

// DeleteSong is the resolver for the deleteSong field.
func (r *mutationResolver) DeleteSong(ctx context.Context, input string) (bool, error) {
	panic(fmt.Errorf("not implemented: DeleteSong - deleteSong"))
}

// PlaySong is the resolver for the playSong field.
func (r *mutationResolver) PlaySong(ctx context.Context, input model.SongInput) (bool, error) {
	panic(fmt.Errorf("not implemented: PlaySong - playSong"))
}

// StopSong is the resolver for the stopSong field.
func (r *mutationResolver) StopSong(ctx context.Context, input *model.SongInput) (bool, error) {
	panic(fmt.Errorf("not implemented: StopSong - stopSong"))
}

// EditConfig is the resolver for the editConfig field.
func (r *mutationResolver) EditConfig(ctx context.Context, input model.ConfigInput) (*model.Config, error) {
	panic(fmt.Errorf("not implemented: EditConfig - editConfig"))
}

// Links is the resolver for the links field.
func (r *queryResolver) Links(ctx context.Context) ([]*model.Link, error) {
	return []*model.Link{
		{
			ID: "link id",
		},
	}, nil
	// panic(fmt.Errorf("not implemented: Links - links"))
}

// Songs is the resolver for the songs field.
func (r *queryResolver) Songs(ctx context.Context) ([]*model.Song, error) {
	dbsongs, err := r.Db.ListSongs()
	songs := make([]*model.Song, 0, len(dbsongs))
	if err != nil {
		// s.httpError(w, fmt.Errorf("ListSongHandler|ListSongs|%w", err), http.StatusBadRequest)
		return nil, err
	}

	for _, s := range dbsongs {
		songs = append(songs, &model.Song{
			ID:       s.ID,
			Title:    &s.Title,
			Rfid:     &s.RFID,
			URL:      s.URL,
			Thumb:    &s.Thumbnail,
			FilePath: &s.FilePath,
		})
	}

	return songs, nil
}

// Song is the resolver for the song field.
func (r *queryResolver) Song(ctx context.Context) (*model.Song, error) {
	panic(fmt.Errorf("not implemented: Song - song"))
}

// Config is the resolver for the config field.
func (r *queryResolver) Config(ctx context.Context) (*model.Config, error) {
	panic(fmt.Errorf("not implemented: Config - config"))
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
