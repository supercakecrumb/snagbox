/**
 * Dependency-free, fetch-based SDK for reporting issues to a snagbox server
 * (the WRITE path only). Uses global `fetch`, `FormData`, and `Blob`
 * available in Node 18+ and modern browsers.
 */

export interface Photo {
  filename: string;
  /** Defaults to "application/octet-stream" when omitted. */
  contentType?: string;
  data: Uint8Array | ArrayBuffer | Blob;
}

export interface ReportRequest {
  text?: string;
  photos?: Photo[];
  meta?: Record<string, unknown>;
}

export interface Attachment {
  id: string;
  filename: string;
  mime: string;
  size_bytes: number;
  url: string;
}

export interface Issue {
  id: string;
  project_id: number | null;
  source: string;
  text: string;
  meta: Record<string, unknown>;
  created_at: string;
  attachments: Attachment[];
}

export class SnagboxError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "SnagboxError";
    this.status = status;
    Object.setPrototypeOf(this, SnagboxError.prototype);
  }
}

export interface SnagboxClientOptions {
  /** Injectable for testing; defaults to the global `fetch`. */
  fetch?: typeof fetch;
}

interface ErrorBody {
  error?: string;
}

export class SnagboxClient {
  private readonly baseURL: string;
  private readonly token: string;
  private readonly _fetch: typeof fetch;

  constructor(baseURL: string, token: string, options?: SnagboxClientOptions) {
    this.baseURL = baseURL.replace(/\/+$/, "");
    this.token = token;
    this._fetch = options?.fetch ?? fetch;
  }

  async reportIssue(req: ReportRequest): Promise<Issue> {
    const hasText = req.text !== undefined && req.text !== "";
    const hasPhotos = req.photos !== undefined && req.photos.length > 0;

    if (!hasText && !hasPhotos) {
      throw new Error("snagbox: empty issue (no text or photos)");
    }

    const url = `${this.baseURL}/api/v1/issues`;
    const headers: Record<string, string> = {
      Authorization: `Bearer ${this.token}`,
    };

    let body: BodyInit;

    if (req.photos && req.photos.length > 0) {
      const form = new FormData();
      form.append("text", req.text ?? "");
      if (req.meta !== undefined) {
        form.append("meta", JSON.stringify(req.meta));
      }
      for (const photo of req.photos) {
        const blob =
          photo.data instanceof Blob
            ? photo.data
            : new Blob([photo.data as BlobPart], {
                type: photo.contentType ?? "application/octet-stream",
              });
        form.append("photo", blob, photo.filename);
      }
      body = form;
      // Do NOT set Content-Type manually — fetch sets the multipart boundary.
    } else {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify({
        text: req.text ?? "",
        ...(req.meta !== undefined ? { meta: req.meta } : {}),
      });
    }

    const res = await this._fetch(url, {
      method: "POST",
      headers,
      body,
    });

    if (res.ok) {
      return (await res.json()) as Issue;
    }

    let message = res.statusText;
    try {
      const parsed = (await res.json()) as ErrorBody;
      if (parsed?.error) {
        message = parsed.error;
      }
    } catch {
      // body wasn't JSON; fall back to statusText
    }

    throw new SnagboxError(res.status, message);
  }
}
