import { Context } from "netlify:edge";

const schemaVersion = 1;
const label = "OPA";
const releases = "https://api.github.com/repos/open-policy-agent/opa/releases/latest";
const endpoint = "/badge-endpoint/";

// this is static/img/opa-logo.svg
const logoSvg = '<?xml version="1.0" encoding="UTF-8" standalone="no"?><!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><svg width="100%" height="100%" viewBox="0 0 34 34" version="1.1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xml:space="preserve" style="fill-rule:evenodd;clip-rule:evenodd;stroke-linejoin:round;stroke-miterlimit:1.41421;"><g><path d="M7.988,0.343c0,0 -1.84,6.196 -1.71,8.032c0.092,1.292 2.493,2.982 2.493,2.982c0,0 -1.308,1.552 -1.813,2.423c-0.521,0.898 -1.313,2.963 -1.313,2.963c0,0 -3.938,-4.622 -3.614,-6.89c0.391,-2.734 5.957,-9.51 5.957,-9.51Z" style="fill:#bfbfbf;"/><path d="M25.857,0.343c0,0 1.84,6.196 1.71,8.032c-0.092,1.292 -2.493,2.982 -2.493,2.982c0,0 1.307,1.552 1.813,2.423c0.521,0.898 1.313,2.963 1.313,2.963c0,0 3.938,-4.622 3.614,-6.89c-0.391,-2.734 -5.957,-9.51 -5.957,-9.51Z" style="fill:#bfbfbf;"/><path d="M16.984,7.6c-5.216,0 -9.819,3.695 -11.344,9.106l11.344,3.763l0,-12.869Z" style="fill:#7d9199;"/><path d="M16.974,7.58c5.215,0 9.819,3.696 11.343,9.107l-11.343,3.762l0,-12.869Z" style="fill:#566366;"/><path d="M16.954,16.7l-11.336,0l0,7.923c0,0 4.577,1.795 6.467,3.245c1.569,1.204 4.588,5.459 4.588,5.459l0.281,-0.002l0,-16.625Z" style="fill:#7d9199;"/><path d="M16.96,16.59l11.337,0l0,7.923c0,0 -4.578,1.795 -6.467,3.245c-1.612,1.238 -4.601,5.567 -4.601,5.567l-0.275,0l0.006,-16.735Z" style="fill:#566366;"/><circle cx="16.963" cy="16.32" r="1.427" style="fill:#fff;"/></g></svg>';

export default async (req: Request, context: Context) => {
  const url = new URL(req.url);

  const version = url.pathname.slice(endpoint.length); // "/badge-endpoint/v0.46.1" => "v0.46.1"

  const latest = await fetch(releases)
    .then((response) => response.json())
    .then((data) => data.tag_name);

  const res = {
    schemaVersion,
    label,
    logoSvg,
    message: version,
    color: latest == version ? "green" : "yellow",
  };

  return context.json(res);
};