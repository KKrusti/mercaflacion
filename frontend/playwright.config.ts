import { defineConfig, devices } from '@playwright/test';

// Required shared libraries not installed system-wide; extracted from deb packages.
const PLAYWRIGHT_LIBS = '/home/carlos/.local/lib/playwright-libs';
const ldLibraryPath = process.env.LD_LIBRARY_PATH
  ? `${PLAYWRIGHT_LIBS}:${process.env.LD_LIBRARY_PATH}`
  : PLAYWRIGHT_LIBS;

// WSLg provides X11/Wayland sockets but doesn't always export DISPLAY into the
// shell that runs `task`. Fall back to the WSLg defaults when the variables are
// missing so that headed mode works without manual export.
const display = process.env.DISPLAY || ':0';
const waylandDisplay = process.env.WAYLAND_DISPLAY || 'wayland-0';
const xdgRuntimeDir = process.env.XDG_RUNTIME_DIR || '/mnt/wslg/runtime-dir';

// In headed mode, slow down actions so interactions are visible.
// Override with SLOW_MO=<ms> env var (e.g. SLOW_MO=200 for faster observation).
const slowMo = process.env.SLOW_MO !== undefined
  ? parseInt(process.env.SLOW_MO, 10)
  : process.env.PW_HEADED === '1' ? 600 : 0;

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: [['list'], ['html', { open: 'never' }]],

  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    launchOptions: {
      slowMo,
      env: {
        LD_LIBRARY_PATH: ldLibraryPath,
        DISPLAY: display,
        WAYLAND_DISPLAY: waylandDisplay,
        XDG_RUNTIME_DIR: xdgRuntimeDir,
      },
    },
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'mobile-poco-x6-pro',
      use: {
        ...devices['Pixel 5'],
        viewport: { width: 393, height: 852 },
        userAgent:
          'Mozilla/5.0 (Linux; Android 14; Poco X6 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36',
        isMobile: true,
        hasTouch: true,
      },
    },
  ],

  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    // Propagate PATH so npm/node are found when spawned as a subprocess.
    // Needed when node is installed in a custom location (e.g. ~/node/bin).
    env: {
      PATH: process.env.PATH ?? '',
    },
  },
});
