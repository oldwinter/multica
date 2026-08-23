/**
 * TestApiClient — lightweight API helper for E2E test data setup/teardown.
 *
 * Uses raw fetch so E2E tests have zero build-time coupling to the web app.
 */

import "./env";
import { createHash, randomBytes } from "node:crypto";
import pg from "pg";

// `||` (not `??`) so an empty `NEXT_PUBLIC_API_URL=` in .env still falls
// back to localhost. dotenv sets unset-vs-empty both as "" — treating them
// the same matches user intent.
const API_BASE = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";

interface TestWorkspace {
  id: string;
  name: string;
  slug: string;
}

export interface TestLMWikiRevision {
  id: string;
  revision_number: number;
  schema_version: number;
  source_digest: string;
  source_policy_version: number;
  source_policy_digest: string;
  remote_generation_enabled: boolean;
  content: {
    schema_version: number;
    egress_policy: {
      remote_generation_enabled: boolean;
      policy_version: number;
      policy_digest: string;
    };
    wiki_pages: TestLMWikiPageEvidence[];
    [key: string]: unknown;
  };
  review: { decision: string } | null;
}

export interface TestLMWikiPageEvidence {
  citation_key: string;
  revision_id: string;
  page_id: string;
  scope: "workspace" | "project";
  project_id?: string;
  revision_number: number;
  path: string;
  title: string;
  content: string;
  content_digest: string;
  created_at?: string;
}

export interface TestLMWikiCitation {
  citation_key: string;
  source_type: string;
  source_id: string;
  source_digest: string;
}

export interface TestLMWikiRevisionDetail {
  revision: TestLMWikiRevision;
  citations: TestLMWikiCitation[];
}

export interface TestWikiPage {
  id: string;
  scope: "workspace" | "project" | "user";
  path: string;
  title: string;
  content: string;
  current_revision_number: number;
  current_revision_id: string;
  content_digest: string;
}

export interface TestWikiRevision {
  id: string;
  page_id: string;
  revision_number: number;
  content: string;
  content_digest: string;
  source_kind: string;
}

export interface TestWikiProposal {
  id: string;
  page_id: string;
  base_revision_number: number;
  proposed_content: string;
  status: "pending" | "accepted" | "rejected";
  accepted_revision_id: string | null;
}

export interface TestLMWikiSourcePolicy {
  source_classes: string[];
  wiki_pages: Array<{ page_id: string; revision_number: number }>;
  remote_generation_enabled: boolean;
  policy_version: number;
  policy_digest: string;
  exclusions: Array<{
    source_class: string;
    state: string;
    reason: string;
  }>;
}

export type TestLMWikiSourcePolicyInput = Pick<
  TestLMWikiSourcePolicy,
  "source_classes" | "wiki_pages" | "remote_generation_enabled"
>;

export interface TestLMWikiOverview {
  latest_revision: TestLMWikiRevision | null;
  accepted_revision: TestLMWikiRevision | null;
  pending_revision: TestLMWikiRevision | null;
  revisions: TestLMWikiRevision[];
  can_manage: boolean;
}

export interface TestTwinProposal {
  id: string;
  kind: string;
  source_wiki_revision_id: string;
  review: { decision: string } | null;
}

export interface TestTwinVersion {
  id: string;
  version_number: number;
  proposal_id: string;
  source_wiki_revision_id: string;
}

export interface TestTwinOverview {
  current_version: TestTwinVersion | null;
  pending_proposal: TestTwinProposal | null;
  proposals: TestTwinProposal[];
  versions: TestTwinVersion[];
  can_manage: boolean;
}

export interface TestWikiTwinArtifactCounts {
  workspace_wiki_pages: number;
  workspace_wiki_revisions: number;
  workspace_wiki_proposals: number;
  wiki_source_policies: number;
  wiki_source_selections: number;
  wiki_revisions: number;
  wiki_citations: number;
  wiki_reviews: number;
  twin_proposals: number;
  twin_reviews: number;
  twin_versions: number;
}

export type TestIssueStatus =
  | "backlog"
  | "todo"
  | "in_progress"
  | "in_review"
  | "done"
  | "blocked"
  | "cancelled";

export type TestIssuePriority = "urgent" | "high" | "medium" | "low" | "none";

export interface TestTableIssueSeed {
  title: string;
  status?: TestIssueStatus;
  priority?: TestIssuePriority;
  parentIssueId?: string | null;
  position?: number;
}

export interface TestTableIssue {
  id: string;
  title: string;
  status: TestIssueStatus;
  number: number;
}

export class TestApiClient {
  private token: string | null = null;
  private workspaceSlug: string | null = null;
  private workspaceId: string | null = null;
  private email: string | null = null;
  private createdIssueIds: string[] = [];
  private seededIssueIds: string[] = [];

  async login(email: string, name: string) {
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      // Keep each E2E login isolated so previous test runs do not trip the
      // per-email send-code rate limit.
      await client.query("DELETE FROM verification_code WHERE email = $1", [email]);

      // Step 1: Send verification code
      const sendRes = await fetch(`${API_BASE}/auth/send-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      if (!sendRes.ok) {
        throw new Error(`send-code failed: ${sendRes.status}`);
      }

      // Step 2: Read code from database
      const result = await client.query(
        "SELECT code FROM verification_code WHERE email = $1 AND used = FALSE AND expires_at > now() ORDER BY created_at DESC LIMIT 1",
        [email],
      );
      if (result.rows.length === 0) {
        throw new Error(`No verification code found for ${email}`);
      }

      const configuredDevCode = process.env.MULTICA_DEV_VERIFICATION_CODE?.trim();
      const code = configuredDevCode || result.rows[0].code;

      // Step 3: Verify code to get JWT
      const verifyRes = await fetch(`${API_BASE}/auth/verify-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, code }),
      });
      if (!verifyRes.ok) {
        throw new Error(`verify-code failed: ${verifyRes.status}`);
      }
      const data = await verifyRes.json();

      this.token = data.token;
      this.email = email;

      // Update user name if needed
      if (name && data.user?.name !== name) {
        await this.authedFetch("/api/me", {
          method: "PATCH",
          body: JSON.stringify({ name }),
        });
      }

      await client.query("DELETE FROM verification_code WHERE email = $1", [email]);

      return data;
    } finally {
      await client.end();
    }
  }

  async getWorkspaces(): Promise<TestWorkspace[]> {
    const res = await this.authedFetch("/api/workspaces");
    return res.json();
  }

  async resetAppearancePreferences(): Promise<void> {
    const current = await this.jsonRequest<{
      appearance_updated_at?: string | null;
    }>("/api/me");
    const previous = Date.parse(current.appearance_updated_at ?? "");
    const updatedAt = new Date(
      Math.max(Date.now(), Number.isFinite(previous) ? previous + 1 : 0),
    ).toISOString();
    await this.jsonRequest("/api/me", {
      method: "PATCH",
      body: JSON.stringify({
        skin: "tension",
        appearance: "system",
        appearance_updated_at: updatedAt,
        appearance_token_version: 1,
      }),
    });
  }

  setWorkspaceId(id: string) {
    this.workspaceId = id;
  }

  setWorkspaceSlug(slug: string) {
    this.workspaceSlug = slug;
  }

  async ensureWorkspace(name = "E2E Workspace", slug = "e2e-workspace") {
    const workspaces = await this.getWorkspaces();
    const workspace = workspaces.find((item) => item.slug === slug) ?? workspaces[0];
    if (workspace) {
      this.workspaceId = workspace.id;
      this.workspaceSlug = workspace.slug;
      return workspace;
    }

    const res = await this.authedFetch("/api/workspaces", {
      method: "POST",
      body: JSON.stringify({ name, slug }),
    });
    if (res.ok) {
      const created = (await res.json()) as TestWorkspace;
      this.workspaceId = created.id;
      this.workspaceSlug = created.slug;
      return created;
    }

    const refreshed = await this.getWorkspaces();
    const created = refreshed.find((item) => item.slug === slug) ?? refreshed[0];
    if (created) {
      this.workspaceId = created.id;
      this.workspaceSlug = created.slug;
      return created;
    }

    throw new Error(`Failed to ensure workspace ${slug}: ${res.status} ${res.statusText}`);
  }

  async markUserOnboarded() {
    if (!this.email) {
      throw new Error("Cannot mark E2E user onboarded before login");
    }

    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query(
        `
          UPDATE "user"
          SET
            onboarded_at = COALESCE(onboarded_at, now()),
            onboarding_questionnaire = COALESCE(onboarding_questionnaire, '{}'::jsonb)
              || '{"source":["friends_colleagues"],"source_other":null,"source_skipped":false}'::jsonb
          WHERE email = $1
        `,
        [this.email],
      );
      if (result.rowCount !== 1) {
        throw new Error(`Failed to mark E2E user onboarded: ${this.email}`);
      }
    } finally {
      await client.end();
    }
  }

  async createIssue(title: string, opts?: Record<string, unknown>) {
    const res = await this.authedFetch("/api/issues", {
      method: "POST",
      body: JSON.stringify({ title, ...opts }),
    });
    const issue = await res.json();
    this.createdIssueIds.push(issue.id);
    return issue;
  }

  /**
   * Insert a large, deterministic issue fixture in one transaction.
   *
   * Browser E2E coverage for cursor-backed Table views needs 1,000+ rows,
   * which would make setup itself dominate the test if every row went through
   * the HTTP create endpoint. These rows intentionally contain no dependent
   * records; cleanup deletes exactly the returned IDs from the isolated E2E
   * workspace.
   */
  async seedTableIssues(rows: TestTableIssueSeed[]): Promise<TestTableIssue[]> {
    if (rows.length === 0) return [];
    if (!this.workspaceId || !this.email) {
      throw new Error("Cannot seed table issues before login and workspace setup");
    }

    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      await client.query("BEGIN");
      const userResult = await client.query<{ id: string }>(
        `SELECT id FROM "user" WHERE email = $1`,
        [this.email],
      );
      const creatorId = userResult.rows[0]?.id;
      if (!creatorId) {
        throw new Error(`Cannot resolve E2E creator for ${this.email}`);
      }

      const counterResult = await client.query<{ issue_counter: number }>(
        `
          UPDATE workspace
          SET issue_counter = issue_counter + $2
          WHERE id = $1
          RETURNING issue_counter
        `,
        [this.workspaceId, rows.length],
      );
      const finalCounter = Number(counterResult.rows[0]?.issue_counter);
      if (!Number.isFinite(finalCounter)) {
        throw new Error(`Cannot reserve issue numbers for workspace ${this.workspaceId}`);
      }
      const firstNumber = finalCounter - rows.length + 1;

      const inserted = await client.query<TestTableIssue>(
        `
          INSERT INTO issue (
            workspace_id,
            title,
            status,
            priority,
            creator_type,
            creator_id,
            parent_issue_id,
            position,
            number
          )
          SELECT
            $1::uuid,
            fixture.title,
            fixture.status,
            fixture.priority,
            'member',
            $2::uuid,
            fixture.parent_issue_id,
            fixture.position,
            fixture.number
          FROM unnest(
            $3::text[],
            $4::text[],
            $5::text[],
            $6::uuid[],
            $7::double precision[],
            $8::integer[]
          ) WITH ORDINALITY AS fixture(
            title,
            status,
            priority,
            parent_issue_id,
            position,
            number,
            ordinal
          )
          ORDER BY fixture.ordinal
          RETURNING id, title, status, number
        `,
        [
          this.workspaceId,
          creatorId,
          rows.map((row) => row.title),
          rows.map((row) => row.status ?? "backlog"),
          rows.map((row) => row.priority ?? "none"),
          rows.map((row) => row.parentIssueId ?? null),
          rows.map((row, index) => row.position ?? index + 1),
          rows.map((_row, index) => firstNumber + index),
        ],
      );
      await client.query("COMMIT");
      this.seededIssueIds.push(...inserted.rows.map((row) => row.id));
      return inserted.rows;
    } catch (error) {
      await client.query("ROLLBACK");
      throw error;
    } finally {
      await client.end();
    }
  }

  async deleteIssue(id: string) {
    await this.authedFetch(`/api/issues/${id}`, { method: "DELETE" });
  }

  async updateIssue(id: string, updates: Record<string, unknown>) {
    const res = await this.authedFetch(`/api/issues/${id}`, {
      method: "PUT",
      body: JSON.stringify(updates),
    });
    if (!res.ok) {
      throw new Error(`update issue failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
  }

  async createWikiPage(input: {
    scope: "workspace" | "project" | "user";
    project_id?: string;
    path: string;
    title: string;
    content: string;
  }): Promise<TestWikiPage> {
    return this.jsonRequest("/api/wiki/pages/", {
      method: "POST",
      body: JSON.stringify(input),
    });
  }

  async updateWikiPage(
    pageId: string,
    input: { expected_revision_number: number; path?: string; title?: string; content?: string },
  ): Promise<TestWikiPage> {
    return this.jsonRequest(`/api/wiki/pages/${pageId}/`, {
      method: "PUT",
      body: JSON.stringify(input),
    });
  }

  async deleteWikiPage(pageId: string): Promise<void> {
    const response = await this.authedFetch(`/api/wiki/pages/${pageId}/`, { method: "DELETE" });
    if (!response.ok) {
      throw new Error(`DELETE Wiki page failed: ${response.status} ${await response.text()}`);
    }
  }

  async deletePersonalWikiPage(pageId: string): Promise<void> {
    const response = await this.authedFetch(`/api/personal-wiki/pages/${pageId}`, {
      method: "DELETE",
    });
    if (!response.ok) {
      throw new Error(
        `DELETE Personal Wiki page failed: ${response.status} ${await response.text()}`,
      );
    }
  }

  async getStableWikiRevision(revisionId: string): Promise<TestWikiRevision> {
    return this.jsonRequest(`/api/wiki/revisions/${revisionId}`);
  }

  async searchWikiPages(query: string, scope = "all"): Promise<TestWikiPage[]> {
    return this.jsonRequest(`/api/wiki/search?q=${encodeURIComponent(query)}&scope=${encodeURIComponent(scope)}`);
  }

  async listWikiRevisions(pageId: string): Promise<TestWikiRevision[]> {
    return this.jsonRequest(`/api/wiki/pages/${pageId}/revisions`);
  }

  async getLMWikiSourcePolicy(): Promise<TestLMWikiSourcePolicy> {
    return this.jsonRequest("/api/lm-wiki/source-policy");
  }

  async updateLMWikiSourcePolicy(policy: TestLMWikiSourcePolicyInput): Promise<TestLMWikiSourcePolicy> {
    return this.jsonRequest("/api/lm-wiki/source-policy", {
      method: "PUT",
      body: JSON.stringify(policy),
    });
  }

  async createWikiAgentCredential(
    issueId: string,
  ): Promise<{ agentId: string; taskId: string; taskToken: string }> {
    if (!this.workspaceId || !this.email) {
      throw new Error("Cannot create a Wiki Agent credential before login and workspace setup");
    }
    const taskToken = `mat_${randomBytes(20).toString("hex")}`;
    const tokenHash = createHash("sha256").update(taskToken).digest("hex");
    const suffix = randomBytes(8).toString("hex");
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      await client.query("BEGIN");
      const user = await client.query<{ id: string }>(
        `SELECT id::text FROM "user" WHERE email = $1`,
        [this.email],
      );
      const userId = user.rows[0]?.id;
      if (!userId) throw new Error(`Cannot resolve Wiki E2E user ${this.email}`);
      const runtime = await client.query<{ id: string }>(
        `INSERT INTO agent_runtime (
           workspace_id, owner_id, name, runtime_mode, provider, status,
           visibility, device_info, metadata, last_seen_at
         ) VALUES ($1, $2, $3, 'cloud', $4, 'online',
                   'private', 'Wiki knowledge E2E', '{}'::jsonb, now())
         RETURNING id::text`,
        [this.workspaceId, userId, `Wiki knowledge runtime ${suffix}`, `wiki_e2e_${suffix}`],
      );
      const runtimeId = runtime.rows[0]?.id;
      if (!runtimeId) throw new Error("Cannot create Wiki E2E runtime");
      const agent = await client.query<{ id: string }>(
        `INSERT INTO agent (
           workspace_id, name, description, instructions, runtime_mode,
           runtime_config, runtime_id, visibility, max_concurrent_tasks, owner_id
         ) VALUES ($1, $2, 'Proposes reviewed Wiki edits', '', 'cloud',
                   '{}'::jsonb, $3, 'workspace', 1, $4)
         RETURNING id::text`,
        [this.workspaceId, `Wiki proposal Agent ${suffix}`, runtimeId, userId],
      );
      const agentId = agent.rows[0]?.id;
      if (!agentId) throw new Error("Cannot create Wiki E2E Agent");
      const task = await client.query<{ id: string }>(
        `INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority)
         VALUES ($1, $2, $3, 'queued', 1)
         RETURNING id::text`,
        [agentId, runtimeId, issueId],
      );
      const taskId = task.rows[0]?.id;
      if (!taskId) throw new Error("Cannot create Wiki E2E task");
      await client.query(
        `INSERT INTO task_token (token_hash, task_id, agent_id, workspace_id, user_id, expires_at)
         VALUES ($1, $2, $3, $4, $5, now() + interval '15 minutes')`,
        [tokenHash, taskId, agentId, this.workspaceId, userId],
      );
      await client.query("COMMIT");
      return { agentId, taskId, taskToken };
    } catch (error) {
      await client.query("ROLLBACK");
      throw error;
    } finally {
      await client.end();
    }
  }

  requestWithTaskToken(path: string, taskToken: string, init?: RequestInit) {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((init?.headers as Record<string, string>) ?? {}),
      Authorization: `Bearer ${taskToken}`,
    };
    if (this.workspaceSlug) headers["X-Workspace-Slug"] = this.workspaceSlug;
    else if (this.workspaceId) headers["X-Workspace-ID"] = this.workspaceId;
    return fetch(`${API_BASE}${path}`, { ...init, headers });
  }

  async getLMWiki(): Promise<TestLMWikiOverview> {
    return this.jsonRequest("/api/lm-wiki/");
  }

  async getLMWikiRevision(revisionId: string): Promise<TestLMWikiRevisionDetail> {
    return this.jsonRequest(`/api/lm-wiki/revisions/${revisionId}`);
  }

  async refreshLMWiki(): Promise<{ created: boolean; revision: TestLMWikiRevision }> {
    return this.jsonRequest("/api/lm-wiki/refresh", { method: "POST" });
  }

  async acceptLMWikiRevision(
    revisionId: string,
  ): Promise<{ revision: TestLMWikiRevision }> {
    return this.jsonRequest(`/api/lm-wiki/revisions/${revisionId}/accept`, {
      method: "POST",
    });
  }

  async rejectLMWikiRevision(
    revisionId: string,
    reason: string,
  ): Promise<{ revision: TestLMWikiRevision }> {
    return this.jsonRequest(`/api/lm-wiki/revisions/${revisionId}/reject`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  async getTwins(): Promise<TestTwinOverview> {
    return this.jsonRequest("/api/twins/");
  }

  async ensureTwinProposal(
    wikiRevisionId: string,
  ): Promise<{ created: boolean; proposal: TestTwinProposal }> {
    return this.jsonRequest("/api/twins/proposals", {
      method: "POST",
      body: JSON.stringify({ wiki_revision_id: wikiRevisionId }),
    });
  }

  async acceptTwinProposal(
    proposalId: string,
  ): Promise<{ created: boolean; version: TestTwinVersion }> {
    return this.jsonRequest(`/api/twins/proposals/${proposalId}/accept`, {
      method: "POST",
    });
  }

  async rejectTwinProposal(proposalId: string, reason: string): Promise<void> {
    await this.jsonRequest(`/api/twins/proposals/${proposalId}/reject`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  request(path: string, init?: RequestInit) {
    return this.authedFetch(path, init);
  }

  async deleteWorkspace(id: string) {
    const res = await this.authedFetch(`/api/workspaces/${id}`, { method: "DELETE" });
    if (!res.ok) {
      throw new Error(`delete workspace failed: ${res.status} ${await res.text()}`);
    }
    if (this.workspaceId === id) {
      this.workspaceId = null;
      this.workspaceSlug = null;
    }
  }

  async getWikiTwinArtifactCounts(workspaceId: string): Promise<TestWikiTwinArtifactCounts> {
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query<TestWikiTwinArtifactCounts>(
        `
          SELECT
            (SELECT count(*)::int FROM wiki_page WHERE workspace_id = $1) AS workspace_wiki_pages,
            (SELECT count(*)::int FROM wiki_page_revision WHERE workspace_id = $1) AS workspace_wiki_revisions,
            (SELECT count(*)::int FROM wiki_page_edit_proposal WHERE workspace_id = $1) AS workspace_wiki_proposals,
            (SELECT count(*)::int FROM lm_wiki_source_policy WHERE workspace_id = $1) AS wiki_source_policies,
            (SELECT count(*)::int FROM lm_wiki_source_wiki_page WHERE workspace_id = $1) AS wiki_source_selections,
            (SELECT count(*)::int FROM lm_wiki_revision WHERE workspace_id = $1) AS wiki_revisions,
            (SELECT count(*)::int FROM lm_wiki_citation WHERE workspace_id = $1) AS wiki_citations,
            (SELECT count(*)::int FROM lm_wiki_review WHERE workspace_id = $1) AS wiki_reviews,
            (SELECT count(*)::int FROM twin_proposal WHERE workspace_id = $1) AS twin_proposals,
            (SELECT count(*)::int FROM twin_proposal_review WHERE workspace_id = $1) AS twin_reviews,
            (SELECT count(*)::int FROM twin_version WHERE workspace_id = $1) AS twin_versions
        `,
        [workspaceId],
      );
      const counts = result.rows[0];
      if (!counts) throw new Error(`No Wiki/Twin cleanup count returned for ${workspaceId}`);
      return counts;
    } finally {
      await client.end();
    }
  }

  async deleteUser() {
    if (!this.email) return;
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      await client.query(`DELETE FROM "user" WHERE email = $1`, [this.email]);
      this.token = null;
      this.email = null;
    } finally {
      await client.end();
    }
  }

  /** Clean up all issues created during this test. */
  async cleanup() {
    if (this.seededIssueIds.length > 0 && this.workspaceId) {
      const client = new pg.Client(DATABASE_URL);
      await client.connect();
      try {
        await client.query(
          `DELETE FROM issue WHERE workspace_id = $1 AND id = ANY($2::uuid[])`,
          [this.workspaceId, this.seededIssueIds],
        );
      } finally {
        await client.end();
      }
      this.seededIssueIds = [];
    }
    for (const id of this.createdIssueIds) {
      try {
        await this.deleteIssue(id);
      } catch {
        /* ignore — may already be deleted */
      }
    }
    this.createdIssueIds = [];
  }

  getToken() {
    return this.token;
  }

  getEmail() {
    if (!this.email) {
      throw new Error("Test API client is not logged in");
    }
    return this.email;
  }

  private async jsonRequest<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await this.authedFetch(path, init);
    if (!res.ok) {
      throw new Error(`${init?.method ?? "GET"} ${path} failed: ${res.status} ${await res.text()}`);
    }
    const body: T = await res.json();
    return body;
  }

  private async authedFetch(path: string, init?: RequestInit) {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((init?.headers as Record<string, string>) ?? {}),
    };
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`;
    if (this.workspaceSlug) headers["X-Workspace-Slug"] = this.workspaceSlug;
    else if (this.workspaceId) headers["X-Workspace-ID"] = this.workspaceId;
    return fetch(`${API_BASE}${path}`, { ...init, headers });
  }
}
