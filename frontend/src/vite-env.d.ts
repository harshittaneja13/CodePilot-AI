/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** When 'true', the app serves built-in mock data (no backend). Set for static demo deploys. */
  readonly VITE_DEMO_MODE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
