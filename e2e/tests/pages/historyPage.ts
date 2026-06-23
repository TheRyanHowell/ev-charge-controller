import { Page, Locator } from "@playwright/test";
import { BasePage } from "./basePage";

export class HistoryPage extends BasePage {
  public readonly pageTitle: Locator;
  public readonly datePicker: Locator;
  public readonly vehicleFilter: Locator;
  public readonly sessionCards: Locator;
  public readonly backToDashboardLink: Locator;
  public readonly loadMoreButton: Locator;
  public readonly emptyState: Locator;

  constructor(page: Page) {
    super(page);
    this.pageTitle = page.getByRole("heading", { name: /charge history/i });
    this.datePicker = page.getByLabel(/filter by date/i).first();
    this.vehicleFilter = page.getByRole("combobox").first();
    // Session cards are expandable buttons containing vehicle name + status
    this.sessionCards = page.getByRole("button").filter({
      hasText: /completed|active|cancelled|stopped/i,
    });
    this.backToDashboardLink = page
      .getByRole("link", {
        name: /back to dashboard/i,
      })
      .first();
    this.loadMoreButton = page.getByTestId("load-more").first();
    this.emptyState = page.getByText(/no charge sessions yet/i).first();
  }

  public async goto() {
    await super.goto("/history");
    await this.page.waitForURL(/\/history/, { timeout: 15_000 });
    await this.page.waitForLoadState("domcontentloaded");
  }

  public getSessionCard(index: number): Locator {
    return this.sessionCards.nth(index);
  }

  public getDeleteButton(index: number): Locator {
    return this.page
      .getByRole("button", { name: /delete.*session/i })
      .nth(index);
  }

  public async getSessionCount(): Promise<number> {
    return this.sessionCards.count();
  }

  public async toggleSessionCard(index: number) {
    const card = this.getSessionCard(index);
    await card.click();
  }

  public async expandFirstSession() {
    const count = await this.getSessionCount();
    if (count > 0) {
      await this.toggleSessionCard(0);
    }
  }

  public async selectVehicleFilter(vehicleName: string) {
    await this.vehicleFilter.selectOption({ label: vehicleName });
  }

  public async selectAllVehicles() {
    await this.vehicleFilter.selectOption({ label: "All Vehicles" });
  }

  public async isSessionExpanded(index: number): Promise<boolean> {
    const card = this.getSessionCard(index);
    const expanded = await card.getAttribute("aria-expanded");
    return expanded === "true";
  }

  public async hasEmptyState(): Promise<boolean> {
    return this.emptyState.isVisible();
  }

  public async goToDashboard() {
    await this.backToDashboardLink.click();
    await this.page.waitForURL(/\/dashboard/, { timeout: 15_000 });
  }

  public async getSessionStatus(index: number): Promise<string | null> {
    const card = this.getSessionCard(index);
    return card.textContent();
  }
}
