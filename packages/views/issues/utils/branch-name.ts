const MAX_BRANCH_NAME_LENGTH = 80;

export function buildIssueBranchName(identifier: string, title: string): string {
  return `${identifier}-${title}`
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/['\u2019]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, MAX_BRANCH_NAME_LENGTH)
    .replace(/-+$/g, "");
}
