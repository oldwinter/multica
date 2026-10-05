import type {
  OfficeRendererStatus,
  OfficeSceneCommit,
  OfficeSceneHandle,
  OfficeScenePort,
} from "./contracts";
import { OfficeSceneController } from "./reconciler";

type CreateIntersectionObserver = (
  callback: IntersectionObserverCallback,
) => IntersectionObserver;

function defaultIntersectionObserverFactory(): CreateIntersectionObserver | null {
  if (typeof IntersectionObserver === "undefined") return null;
  return (callback) =>
    new IntersectionObserver(callback, {
      rootMargin: "120px",
      threshold: 0,
    });
}

export class OfficeSceneRuntime implements OfficeSceneHandle {
  readonly #document: Document;
  readonly #port: OfficeScenePort;
  readonly #controller: OfficeSceneController;
  readonly #onStatus: (status: OfficeRendererStatus) => void;
  readonly #intersectionObserver: IntersectionObserver | null;
  readonly #unsubscribeContextLoss: () => void;
  #onScreen = true;
  #active = false;
  #contextLosses = 0;
  #recovery: Promise<void> = Promise.resolve();
  #destroyed = false;
  #retired = false;

  constructor(input: {
    readonly host: HTMLElement;
    readonly port: OfficeScenePort;
    readonly onStatus: (status: OfficeRendererStatus) => void;
    readonly documentObject?: Document;
    readonly createIntersectionObserver?: CreateIntersectionObserver | null;
  }) {
    this.#document = input.documentObject ?? document;
    this.#port = input.port;
    this.#onStatus = input.onStatus;
    this.#controller = new OfficeSceneController({
      port: input.port,
      onStatus: input.onStatus,
    });
    this.#document.addEventListener(
      "visibilitychange",
      this.#handleVisibilityChange,
    );
    const createObserver =
      input.createIntersectionObserver === undefined
        ? defaultIntersectionObserverFactory()
        : input.createIntersectionObserver;
    this.#intersectionObserver = createObserver
      ? createObserver((entries) => {
          const entry = entries.find((candidate) => candidate.target === input.host);
          if (!entry) return;
          this.#onScreen = entry.isIntersecting;
          this.#syncActivity();
        })
      : null;
    this.#intersectionObserver?.observe(input.host);
    this.#unsubscribeContextLoss = this.#port.onContextLoss(
      this.#handleContextLoss,
    );
    this.#syncActivity();
  }

  readonly #handleVisibilityChange = () => {
    this.#syncActivity();
  };

  readonly #handleContextLoss = () => {
    if (this.#destroyed || this.#retired) return;
    this.#contextLosses += 1;
    if (this.#contextLosses > 1) {
      this.#retireForContextFailure();
      return;
    }

    this.#onStatus({ kind: "recovering" });
    this.#port.pause();
    this.#recovery = this.#port
      .rebuild()
      .then(async () => {
        if (this.#destroyed || this.#retired || this.#contextLosses > 1) return;
        this.#controller.reapplyLatestAsReplace();
        await this.#controller.whenIdle();
        if (this.#active) this.#port.resume();
      })
      .catch(() => {
        this.#retireForContextFailure();
      });
  };

  #retireForContextFailure() {
    if (this.#retired) return;
    this.#retired = true;
    this.#controller.destroy();
    this.#onStatus({ kind: "fallback", reason: "context" });
  }

  #syncActivity() {
    if (this.#destroyed || this.#retired) return;
    const active = !this.#document.hidden && this.#onScreen;
    if (active === this.#active) return;
    this.#active = active;
    if (active) {
      this.#port.resume();
      this.#controller.reapplyLatestAsReplace();
    } else {
      this.#port.pause();
    }
  }

  reconcile(commit: OfficeSceneCommit): void {
    if (this.#destroyed || this.#retired) return;
    this.#controller.reconcile(
      this.#active ? commit : { ...commit, mode: "replace", effects: [] },
    );
  }

  fit(): void {
    this.#port.fit();
  }

  zoomIn(): void {
    this.#port.zoomIn();
  }

  zoomOut(): void {
    this.#port.zoomOut();
  }

  async whenIdle(): Promise<void> {
    await this.#recovery;
    await this.#controller.whenIdle();
  }

  destroy(): void {
    if (this.#destroyed) return;
    this.#destroyed = true;
    this.#document.removeEventListener(
      "visibilitychange",
      this.#handleVisibilityChange,
    );
    this.#intersectionObserver?.disconnect();
    this.#unsubscribeContextLoss();
    this.#controller.destroy();
  }
}
