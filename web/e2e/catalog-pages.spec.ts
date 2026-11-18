import {test,expect} from "@playwright/test";
test("catalog pages preserve filters and retry the failed destination",async({page})=>{
 let attempts=0;const cursors:string[]=[];
 const headers={"Access-Control-Allow-Origin":"http://127.0.0.1:3000","Access-Control-Allow-Headers":"Authorization, Content-Type"};
 await page.route("http://localhost:8080/api/v1**",async route=>{
 const u=new URL(route.request().url()),path=u.pathname.replace("/api/v1","");
 const send=(body:unknown,status=200)=>route.fulfill({status,headers,contentType:"application/json",body:JSON.stringify(body)});
 if(route.request().method()==="OPTIONS")return route.fulfill({status:204,headers});
 if(path==="")return send({authenticationRequired:false});
 if(path==="/tracker")return send({items:[]});
 if(path==="/anime"){
  if(!u.searchParams.has("mediaType"))return send({items:[]});
  expect(u.searchParams.get("mediaType")).toBe("TV");
  const cursor=u.searchParams.get("cursor")??"";cursors.push(cursor);
  if(cursor==="page-two"&&attempts++===0)return send({},500);
  return send({items:[{id:"10000000-0000-4000-8000-000000000001",title:cursor==="start"?"Synthetic First Page":"Synthetic Second Page",mediaType:"TV"}],...(cursor==="start"?{nextCursor:"page-two"}:{})});
 }
 return send({},404);
 });
 await page.goto("/");
 await page.getByRole("button",{name:"Catalog filters",exact:true}).click();
 await page.getByLabel("Catalog format",{exact:true}).selectOption("TV");
 await page.getByRole("button",{name:"Apply catalog filters",exact:true}).click();
 await expect(page.getByText("Synthetic First Page",{exact:true})).toBeVisible();
 await page.getByRole("button",{name:"Next catalog page",exact:true}).click();
 await expect(page.getByRole("button",{name:"Retry catalog search",exact:true})).toBeVisible();
 await expect(page.getByText("Synthetic First Page",{exact:true})).toBeVisible();
 await page.getByRole("button",{name:"Retry catalog search",exact:true}).click();
 await expect(page.getByText("Synthetic Second Page",{exact:true})).toBeVisible();
 await page.getByRole("button",{name:"Previous catalog page",exact:true}).click();
 await expect(page.getByText("Synthetic First Page",{exact:true})).toBeVisible();
 expect(cursors).toEqual(["start","page-two","page-two","start"]);
});
