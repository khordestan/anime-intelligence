package store
import("context";"strings";"unicode/utf8")
type CatalogFacetPage struct{Items []string `json:"items"`;HasMore bool `json:"hasMore"`}
type FacetedCatalog interface{CatalogFacets(context.Context,string,string,int)(CatalogFacetPage,error)}
func ValidateFacet(field,prefix string,limit int)error{
 switch field{case "genres","themes","demographics","studios","providers":default:return ErrCatalogFilter}
 if limit<1||limit>100||len(prefix)>128||!utf8.ValidString(prefix)||strings.ContainsRune(prefix,0){return ErrCatalogFilter};return nil
}
func(s Local)CatalogFacets(ctx context.Context,field,prefix string,limit int)(CatalogFacetPage,error){
 out:=CatalogFacetPage{Items:[]string{}}
 if err:=ValidateFacet(field,prefix,limit);err!=nil{return out,err};if err:=ctx.Err();err!=nil{return out,err};return out,nil
}
