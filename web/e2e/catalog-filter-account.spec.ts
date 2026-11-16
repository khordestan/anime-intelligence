import{test,expect}from"@playwright/test";
test("unapplied catalog drafts reset at sign-in and sign-out",async({page})=>{
 const headers={"Access-Control-Allow-Origin":"http://127.0.0.1:3000","Access-Control-Allow-Headers":"Authorization, Content-Type","Access-Control-Allow-Methods":"GET, POST, OPTIONS"};
 await page.route("http://localhost:8080/api/v1**",async route=>{
 const path=new URL(route.request().url()).pathname.replace("/api/v1","");
 const send=(body:unknown)=>route.fulfill({status:200,headers,contentType:"application/json",body:JSON.stringify(body)});
 if(route.request().method()==="OPTIONS")return route.fulfill({status:204,headers});
 if(path==="")return send({authenticationRequired:true,identityDeliveryAvailable:false});
 if(path==="/auth/login")return send({token:"synthetic-token",expiresAt:new Date(Date.now()+86400000).toISOString(),user:{id:"synthetic-account",email:"draft@example.test",displayName:"",emailVerified:false}});
 if(path==="/auth/logout")return route.fulfill({status:204,headers});
 return send({items:[]});
 });
 await page.goto("/");
 await page.getByRole("button",{name:"Catalog filters",exact:true}).click();
 await page.getByLabel("Source genre",{exact:true}).fill("Synthetic anonymous draft");
 await page.getByLabel("Email",{exact:true}).fill("draft@example.test");
 await page.getByLabel("Password",{exact:true}).fill("synthetic-password");
 await page.getByRole("button",{name:"Sign in",exact:true}).click();
 await expect(page.getByText("draft@example.test",{exact:true})).toBeVisible();
 await page.getByRole("button",{name:"Catalog filters",exact:true}).click();
 await expect(page.getByLabel("Source genre",{exact:true})).toHaveValue("");
 await page.getByLabel("Source genre",{exact:true}).fill("Synthetic signed-in draft");
 await page.getByRole("button",{name:"Sign out",exact:true}).click();
 await page.getByRole("button",{name:"Catalog filters",exact:true}).click();
 await expect(page.getByLabel("Source genre",{exact:true})).toHaveValue("");
});
