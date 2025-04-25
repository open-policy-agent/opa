// Redirect path versioned requests to archived versions of the OPA
// documentation.
// Context: https://github.com/open-policy-agent/opa/issues/7037
import { Context } from "@netlify/edge-functions";

export default async (
  request: Request,
  context: Context
): Promise<Response | undefined> => {
  const url: URL = new URL(request.url);
  const pathname: string = url.pathname;

  // this is the name of the archived site project and forms the base of all
  // archived sites.
  const siteName: string = "opa-docs";

  const versionRegex: RegExp = /^\/docs\/v(\d+\.\d+\.\d+)(\/.*)?$/;
  const match: RegExpMatchArray | null = pathname.match(versionRegex);

  if (match) {
    const version: string = match[1]; // e.g., "0.65.0"
    const restOfPath: string = match[2] || "/"; // e.g., "/introduction" or defaults to "/"

    // Format version for the Netlify subdomain: replace dots with dashes
    const formattedVersion: string = version.replace(/\./g, "-"); // e.g., "0-65-0"

    // Construct the target archive URL
    const targetDomain: string = `v${formattedVersion}--${siteName}.netlify.app`;
    const targetUrl: string = `https://${targetDomain}${restOfPath}`;

    console.log(`Edge Function: Redirecting ${pathname} to ${targetUrl}`);

    return Response.redirect(targetUrl, 301);
  }

  return undefined;
};