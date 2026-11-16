import { test, expect } from "@playwright/test";
test("source labels load explicitly and selection only changes filter drafts",async({page})=>{
 const calls:string[]=[];
 const headers={"Access-Control-Allow-Origin":"http://127.0.0.1:3000","Access-Control-Allow-Headers":"Authorization, Content-Type"};
 await page.route("http://localhost:8080/api/v1**",async route=>{
  const u=new URL(route.request().url()),path=u.pathname.replace("/api/v1","");
  const respond=(body:unknown)=>route.fulfill({status:200,headers,contentType:"application/json",body:JSON.stringify(body)});
  if(route.request().method()==="OPTIONS")return route.fulfill({status:204,headers});
  if(path==="")return respond({authenticationRequired:false});
  if(path==="/tracker")return respond({items:[]});
  if(path==="/anime")return respond({items:[]});
  if(path==="/anime/facets"){calls.push(u.searchParams.get("field")??"");return respond({items:u.searchParams.get("field")==="themes"?["Synthetic theme"]:["Action"],hasMore:true});}
  return respond({});
 });
 await page.goto("/");
 await page.getByRole("button",{name:"Catalog filters",exact:true}).click();
 expect(calls).toEqual([]);
 const form=page.getByRole("form",{name:"Catalog metadata filters",exact:true});
 await form.getByRole("button",{name:"Find genres labels",exact:true}).click();
 await form.getByRole("button",{name:"Action",exact:true}).click();
 await expect(form.getByLabel("Source genre",{exact:true})).toHaveValue("Action");
 await expect(page.getByRole("button",{name:"Hide catalog filters",exact:true})).toBeVisible();
 expect(calls).toEqual(["genres"]);
 await form.getByRole("button",{name:"Find theme labels (may contain spoilers)",exact:true}).click();
 await expect(form.getByRole("button",{name:"Synthetic theme",exact:true})).toBeVisible();
 expect(calls).toEqual(["genres","themes"]);
 await form.getByRole("button",{name:"Clear catalog filters",exact:true}).click();
 await expect(form.getByRole("button",{name:"Synthetic theme",exact:true})).toHaveCount(0);
});
