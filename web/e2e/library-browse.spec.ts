import { test, expect } from "@playwright/test";

test("private library browsing combines status, title and rating without changing stored entries", async ({ page }) => {
  const headers = {
    "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
    "Access-Control-Allow-Headers": "Authorization, Content-Type",
    "Access-Control-Allow-Methods": "GET, OPTIONS",
  };
  const entries = [
    {id:"one",animeId:"a",title:"Zeta Synthetic",watchStatus:"WATCHING",episodesWatched:12,rating:5,exactProgress:true,totalEpisodes:20,seasonRatingAvailable:true,seasonRating:4},
    {id:"two",animeId:"b",title:"Alpha Synthetic",watchStatus:"COMPLETED",episodesWatched:24,rating:2,exactProgress:false,seasonRatingAvailable:true},
    {id:"three",animeId:"c",title:"Beta Synthetic",watchStatus:"WATCHING",episodesWatched:2},
    {id:"four",animeId:"d",title:"Delta Synthetic",watchStatus:"WATCHING",episodesWatched:12,rating:5,totalEpisodes:10},
  ];
  const writes: string[] = [];
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const respond = (body: unknown, status=200) => route.fulfill({status,headers,contentType:"application/json",body:JSON.stringify(body)});
    if(request.method()==="OPTIONS") return route.fulfill({status:204,headers});
    if(request.method()!=="GET") writes.push(path);
    if(path==="") return respond({authenticationRequired:false});
    if(path==="/anime") return respond({items:[]});
    if(path==="/tracker") return respond({items:entries});
    return respond({},404);
  });
  await page.goto("/");
  const library = page.getByRole("region",{name:"My Library",exact:true});
  const titles = library.getByRole("article").getByRole("heading",{level:3});
  await expect(titles).toHaveText(entries.map(entry=>entry.title));
  await page.getByLabel("Library sort order",{exact:true}).selectOption("RATING_HIGH");
  await expect(titles).toHaveText(["Delta Synthetic","Zeta Synthetic","Alpha Synthetic","Beta Synthetic"]);
  await page.getByLabel("Library sort order",{exact:true}).selectOption("RATING_LOW");
  await expect(titles).toHaveText(["Alpha Synthetic","Delta Synthetic","Zeta Synthetic","Beta Synthetic"]);
  await page.getByLabel("Library sort order",{exact:true}).selectOption("EPISODES");
  await expect(titles).toHaveText(["Alpha Synthetic","Delta Synthetic","Zeta Synthetic","Beta Synthetic"]);
  await page.getByLabel("Library sort order",{exact:true}).selectOption("TITLE");
  await expect(titles).toHaveText(["Alpha Synthetic","Beta Synthetic","Delta Synthetic","Zeta Synthetic"]);
  await library.getByRole("button",{name:"Watching",exact:true}).click();
  await page.getByLabel("Library rating filter",{exact:true}).selectOption("5");
  await page.getByLabel("Search your library",{exact:true}).fill("  zETA  ");
  await expect(titles).toHaveText(["Zeta Synthetic"]);
  await expect(library.getByRole("status")).toHaveText("Showing 1 of 4 library titles.");
  await library.getByText("Library summary", { exact: true }).click();
  const summary = library.getByRole("region", { name: "Library summary" });
  await expect(summary.getByText("3 of 4 titles rated")).toBeVisible();
  await expect(summary.getByRole("region", { name: "Series ratings" }).getByText("Average 4.0 out of 5")).toBeVisible();
  await expect(summary.getByRole("list", { name: "Rating distribution", exact: true }).getByRole("listitem").last()).toHaveText(/5 \/ 5.*2 titles/);
  await expect(summary.getByText("1 of 4 titles track exact episodes", { exact: true })).toBeVisible();
  await expect(summary.getByText("8 episodes left across 2 titles with recorded totals (3 watching or on hold)", { exact: true })).toBeVisible();
  await expect(summary.getByText("1 of 2 eligible seasons or parts rated", { exact: true })).toBeVisible();
  await expect(summary.getByRole("list", { name: "Season rating distribution" }).getByRole("listitem").nth(3)).toHaveText(/4 \/ 5.*1 title/);
  const comparison = summary.getByRole("region", { name: "Rating comparison" });
  await expect(comparison.getByText("1 title has both ratings")).toBeVisible();
  await expect(comparison.locator("dt")).toHaveText(["Series rating higher", "Season or part rating higher", "Same rating"]);
  await expect(comparison.locator("dd")).toHaveText(["1", "0", "0"]);

  await page.getByLabel("Library rating filter",{exact:true}).selectOption("UNRATED");
  await expect(titles).toHaveCount(0);
  await expect(library.getByText("No library titles match these filters.",{exact:true})).toBeVisible();
  await page.getByLabel("Search your library",{exact:true}).fill("");
  await expect(titles).toHaveText(["Beta Synthetic"]);
  await page.getByLabel("Library rating filter",{exact:true}).selectOption("RATED");
  await expect(titles).toHaveText(["Delta Synthetic","Zeta Synthetic"]);
  await library.getByRole("button",{name:"Clear library filters",exact:true}).click();
  await expect(titles).toHaveText(entries.map(entry=>entry.title));
  await expect(library.getByRole("button",{name:"All",exact:true})).toHaveAttribute("aria-pressed","true");
  await library.getByRole("button",{name:"Refresh",exact:true}).click();
  await expect(titles).toHaveText(entries.map(entry=>entry.title));
  await page.getByLabel("Library progress mode",{exact:true}).selectOption("EXACT");
  await expect(titles).toHaveText(["Zeta Synthetic"]);
  await library.getByRole("button",{name:"Refresh",exact:true}).click();
  await expect(titles).toHaveText(["Zeta Synthetic"]);
  await page.getByLabel("Library rating filter",{exact:true}).selectOption("UNRATED");
  await expect(titles).toHaveCount(0);
  await page.getByLabel("Library progress mode",{exact:true}).selectOption("COUNT");
  await expect(titles).toHaveText(["Beta Synthetic"]);
  await page.getByLabel("Library rating filter",{exact:true}).selectOption("ALL");
  await expect(titles).toHaveText(["Alpha Synthetic","Beta Synthetic","Delta Synthetic"]);
  await library.getByRole("button",{name:"Completed",exact:true}).click();
  await expect(titles).toHaveText(["Alpha Synthetic"]);
  await library.getByRole("button",{name:"Clear library filters",exact:true}).click();
  await expect(page.getByLabel("Library progress mode",{exact:true})).toHaveValue("ALL");
  await expect(titles).toHaveText(entries.map(entry=>entry.title));
  expect(writes).toEqual([]);
});
