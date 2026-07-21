// Redirect legacy *.html URLs to their extensionless equivalents. Runs as an
// edge function because a wildcard _redirects rule can't force a redirect
// away from a file that physically exists at the "from" path.
import { Context } from "@netlify/edge-functions";

const excluded: Set<string> = new Set([
  "/netlify-forms.html",
  "/docs/kubernetes-admission-control.html",
]);

export default async (
  request: Request,
  context: Context,
): Promise<Response | undefined> => {
  const url: URL = new URL(request.url);
  const pathname: string = url.pathname;

  if (!pathname.endsWith(".html") || excluded.has(pathname)) {
    return undefined;
  }

  url.pathname = pathname === "/index.html"
    ? "/"
    : pathname.slice(0, -".html".length);

  return Response.redirect(url.toString(), 301);
};
