import { Page, Locator } from "@playwright/test";
import { BasePage } from "./basePage";

export class DashboardPage extends BasePage {
  public readonly startButton: Locator;
  public readonly stopButton: Locator;
  public readonly settingsButton: Locator;
  public readonly gaugeSvg: Locator;

  constructor(page: Page) {
    super(page);
    this.startButton = page.getByRole("button", { name: /START/i }).first();
    this.stopButton = page.getByRole("button", { name: /STOP/i }).first();
    this.settingsButton = page
      .getByRole("button", { name: /settings/i })
      .first();
    this.gaugeSvg = page.locator("svg").first();
  }

  public async navigateTo() {
    await super.goto("/dashboard");
  }

  public async isReady(): Promise<boolean> {
    return this.gaugeSvg.isVisible();
  }

  public async clickStart() {
    await this.startButton.click();
  }

  public async clickStop() {
    await this.stopButton.click();
  }
}
