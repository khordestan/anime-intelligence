package postgres
import("context";"time";"github.com/jackc/pgx/v5";"github.com/khordestan/anime-intelligence/internal/platform/store")
func(r *Repository)CatalogFacets(ctx context.Context,field,prefix string,limit int)(store.CatalogFacetPage,error){
 out:=store.CatalogFacetPage{Items:[]string{}}
 if err:=store.ValidateFacet(field,prefix,limit);err!=nil{return out,err}
 ctx,cancel:=context.WithTimeout(ctx,3*time.Second);defer cancel()
 tx,err:=r.db.beginTx(ctx,pgx.TxOptions{AccessMode:pgx.ReadOnly});if err!=nil{return out,err};defer rollback(tx)
 if _,err=tx.Exec(ctx,"SET LOCAL statement_timeout='2s'");err!=nil{return out,err}
 query := `SELECT min(btrim(label.value) COLLATE "C") AS label FROM catalog_metadata m
 CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(m.payload->$1::text,'[]'::jsonb)) label(value)
 WHERE btrim(label.value)<>'' AND octet_length(btrim(label.value))<=256
 AND starts_with(lower(btrim(label.value)),lower($2))
 GROUP BY lower(btrim(label.value)) ORDER BY lower(btrim(label.value)) LIMIT $3`
 args := []any{field, prefix, limit+1}
 if field == "providers" {
  query = `SELECT DISTINCT d.provider_key FROM catalog_metadata m JOIN data_sources d ON d.id=m.source_id
   WHERE starts_with(lower(d.provider_key),lower($1))
   ORDER BY d.provider_key LIMIT $2`
  args = []any{prefix, limit+1}
 }
 rows,err:=tx.Query(ctx,query,args...)
 if err!=nil{return out,err}
 for rows.Next(){var value string;if err=rows.Scan(&value);err!=nil{rows.Close();return out,err};out.Items=append(out.Items,value)}
 rows.Close();if err=rows.Err();err!=nil{return out,err}
 if len(out.Items)>limit{out.HasMore=true;out.Items=out.Items[:limit]}
 if err=tx.Commit(ctx);err!=nil{return store.CatalogFacetPage{Items:[]string{}},err};return out,nil
}
