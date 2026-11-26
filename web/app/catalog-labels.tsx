"use client";
import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";
export default function CatalogLabels({client,field,value,onSelect}:{client:ApiClient;field:string;value:string;onSelect:(value:string)=>void}){
 const [items,setItems]=useState<string[]>([]);
 const [busy,setBusy]=useState(false),[error,setError]=useState(""),[more,setMore]=useState(false),[loaded,setLoaded]=useState(false);
 const revision=useRef(0),alive=useRef(true);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;revision.current++;};},[]);
 useEffect(()=>{revision.current++;setItems([]);setLoaded(false);setBusy(false);setError("");setMore(false);},[value]);
 async function load(){
  if(new TextEncoder().encode(value.trim()).length>128){revision.current++;setBusy(false);setItems([]);setLoaded(false);setMore(false);setError("Enter a shorter label prefix (at most 128 UTF-8 bytes).");return;}
  const current=++revision.current;setBusy(true);setError("");
  try{
   const params=new URLSearchParams({field,prefix:value.trim(),limit:"50"});
   const page=await client.request<{items:string[];hasMore:boolean}>("/anime/facets?"+params);
   if(alive.current&&current===revision.current){setItems(page.items);setMore(page.hasMore);setLoaded(true);}
  }catch{if(alive.current&&current===revision.current)setError("Could not load labels. Try again.");}
  finally{if(alive.current&&current===revision.current)setBusy(false);}
 }
 return <div className="mt-1 text-xs">
  <button type="button" disabled={busy} onClick={()=>void load()} className="text-violet-200 underline">{busy?"Loading labels…":field==="themes"?"Find theme labels (may contain spoilers)":field==="providers"?"Find metadata providers":"Find "+field+" labels"}</button>
  {error&&<p role="alert">{error}</p>}
  {loaded&&!items.length&&<p>No stored labels match this prefix.</p>}
  {more&&<p>More labels are available. Enter a narrower prefix and search again.</p>}
  {!!items.length&&<ul aria-label={field+" label suggestions"} className="mt-2 space-y-1">{items.map(item=><li key={item}><button type="button" onClick={()=>onSelect(item)} className="underline">{item}</button></li>)}</ul>}
 </div>;
}
