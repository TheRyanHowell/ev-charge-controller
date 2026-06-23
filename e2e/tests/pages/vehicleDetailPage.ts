import { Page, Locator } from "@playwright/test";
import { BasePage } from "./basePage";

export class VehicleDetailPage extends BasePage {
  public readonly backToVehiclesLink: Locator;
  public readonly editNameButton: Locator;
  public readonly deleteButton: Locator;
  public readonly timeRangeButtons: Locator;
  public readonly statsCards: Locator;
  public readonly energyChart: Locator;
  public readonly vehicleDetails: Locator;
  public readonly deleteConfirmDialog: Locator;

  constructor(page: Page) {
    super(page);
    this.backToVehiclesLink = page.getByRole("link", {
      name: /back to vehicles/i,
    });
    this.editNameButton = page.getByTitle(/edit name/i);
    this.deleteButton = page.getByTitle(/delete/i);
    this.timeRangeButtons = page
      .getByRole("button")
      .and(page.getByText(/week|month|year|lifetime/i));
    this.statsCards = page.locator('[class*="StatCard"]');
    this.energyChart = page.getByText(/daily energy/i);
    this.vehicleDetails = page.getByText(/vehicle details/i);
    this.deleteConfirmDialog = page.getByRole("heading", {
      name: /delete vehicle/i,
    });
  }

  public async navigateTo(vehicleId: string) {
    await super.goto(`/vehicles/${vehicleId}`);
  }

  public getTimeRangeButton(range: string): Locator {
    return this.page
      .getByRole("button")
      .filter({ hasText: new RegExp(`^${range}$`, "i") });
  }

  public async getStatCardValue(label: string): Promise<string | null> {
    const card = this.page
      .locator("div")
      .filter({ hasText: new RegExp(label, "i") });
    const value = card.locator('div[class*="text-lg"]');
    return value.textContent();
  }
}
