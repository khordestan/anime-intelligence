package postgres

import (
 "context"
 "errors"
 "time"
 "unicode/utf8"

 "github.com/jackc/pgx/v5"
)
var ErrNotes=errors.New("invalid tracker notes")
var ErrNotesConflict=errors.New("tracker changed since notes were read")
type TrackerNotes struct{
 Notes string `json:"notes"`
 UpdatedAt time.Time `json:"updatedAt"`
}
func(r *Repository) TrackerNotes(ctx context.Context,id string)(TrackerNotes,bool,error){
 out:=TrackerNotes{}
 tx,u,err:=r.db.productTx(ctx,r.token);if err!=nil{return out,false,err};defer rollback(tx)
 err=tx.QueryRow(ctx,`SELECT COALESCE(notes,''),updated_at FROM user_anime WHERE user_id=$1 AND id=$2 FOR UPDATE`,u.ID,id).Scan(&out.Notes,&out.UpdatedAt)
 if errors.Is(err,pgx.ErrNoRows){return out,false,nil}
 if err!=nil{return out,false,err}
 return out,true,tx.Commit(ctx)
}
// Notes belong to the title's tracker entry, not an individual viewing pass.
// The version is the tracker timestamp, so concurrent progress/rating edits
// conservatively require a refresh too. It is never a credential.
func(r *Repository) SetTrackerNotes(ctx context.Context,id,notes string,expected time.Time)(TrackerNotes,bool,error){
 out:=TrackerNotes{}
 if expected.IsZero()||!utf8.ValidString(notes)||utf8.RuneCountInString(notes)>2000{return out,false,ErrNotes}
 tx,u,err:=r.db.productTx(ctx,r.token);if err!=nil{return out,false,err};defer rollback(tx)
 var current time.Time
 err=tx.QueryRow(ctx,`SELECT updated_at FROM user_anime WHERE user_id=$1 AND id=$2 FOR UPDATE`,u.ID,id).Scan(&current)
 if errors.Is(err,pgx.ErrNoRows){return out,false,nil}
 if err!=nil{return out,false,err}
 if !current.Equal(expected){return out,true,ErrNotesConflict}
 err=tx.QueryRow(ctx,`UPDATE user_anime SET notes=NULLIF($3,''),updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond')
 WHERE user_id=$1 AND id=$2 RETURNING COALESCE(notes,''),updated_at`,u.ID,id,notes).Scan(&out.Notes,&out.UpdatedAt)
 if err!=nil{return out,false,err}
 return out,true,tx.Commit(ctx)
}
