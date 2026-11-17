// Only fixed reviewed links are rendered, never provider-controlled URLs.
export default function ProviderCredit({ provider }: { provider: string }) {
  if (provider !== "tvmaze") return null;
  return <span>
    Data from <a href="https://www.tvmaze.com/" className="underline">TVmaze</a>, adapted under{" "}
    <a href="https://creativecommons.org/licenses/by-sa/4.0/" rel="license" className="underline">CC BY-SA 4.0</a>.
  </span>;
}
