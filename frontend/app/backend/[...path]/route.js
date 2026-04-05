const BACKEND_ORIGIN = process.env.BACKEND_ORIGIN || "http://backend:8080";

export const dynamic = "force-dynamic";

async function forward(request, { params }) {
  const path = Array.isArray(params.path) ? params.path.join("/") : "";
  const target = new URL(`/${path}`, BACKEND_ORIGIN);
  target.search = request.nextUrl.search;

  const headers = new Headers(request.headers);
  headers.delete("host");
  headers.delete("connection");
  headers.delete("content-length");

  const init = {
    method: request.method,
    headers,
    redirect: "manual",
  };

  if (request.method !== "GET" && request.method !== "HEAD") {
    init.body = await request.arrayBuffer();
  }

  let response;
  try {
    response = await fetch(target, init);
  } catch (error) {
    return Response.json(
      { error: `backend unavailable: ${error instanceof Error ? error.message : "request failed"}` },
      { status: 502 },
    );
  }

  const responseHeaders = new Headers(response.headers);
  responseHeaders.delete("content-encoding");
  responseHeaders.delete("content-length");

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: responseHeaders,
  });
}

export { forward as GET, forward as POST, forward as PUT, forward as PATCH, forward as DELETE, forward as OPTIONS };
