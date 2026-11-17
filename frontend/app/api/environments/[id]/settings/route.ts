import { NextRequest, NextResponse } from "next/server";

export async function PUT(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;
  const backend = process.env.BACKEND_URL || "http://localhost:8080";
  const cookie = req.headers.get("cookie") || "";
  const authHeader = req.headers.get("authorization") || "";

  try {
    const body = await req.json().catch(() => ({}));
    const res = await fetch(`${backend}/api/sandboxes/${id}/settings`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        cookie,
        ...(authHeader ? { authorization: authHeader } : {}),
      },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      return NextResponse.json(data, { status: res.status });
    }

    const data = await res.json();
    return NextResponse.json(data);
  } catch (err: unknown) {
    return NextResponse.json(
      { error: (err as Error).message },
      { status: 500 }
    );
  }
}

export async function PATCH(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  return PUT(req, props);
}

