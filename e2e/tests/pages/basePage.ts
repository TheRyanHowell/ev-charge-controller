import { Page } from "@playwright/test";

export class BasePage {
  protected readonly page: Page;

  constructor(page: Page) {
    this.page = page;
  }

  public async goto(path: string) {
    await this.page.goto(path, { waitUntil: "domcontentloaded" });
  }

  public async waitForLoad(
    state: "networkidle" | "domcontentloaded" | "load" = "domcontentloaded",
  ) {
    await this.page.waitForLoadState(state);
  }
}
