import { Page, Locator } from "@playwright/test";
import { BasePage } from "./basePage";

export class VehiclesPage extends BasePage {
  public readonly pageTitle: Locator;
  public readonly addVehicleButton: Locator;
  public readonly vehicleList: Locator;
  public readonly backToDashboardLink: Locator;
  public readonly emptyState: Locator;
  public readonly addDialog: Locator;
  public readonly deleteDialog: Locator;

  constructor(page: Page) {
    super(page);
    this.pageTitle = page.getByRole("heading", { name: /vehicles/i });
    this.addVehicleButton = page
      .getByRole("button", {
        name: /add vehicle/i,
      })
      .first();
    this.vehicleList = page.locator('a[href^="/vehicles/"]');
    this.backToDashboardLink = page
      .getByRole("link", {
        name: /back to dashboard/i,
      })
      .first();
    this.emptyState = page.getByText(/no vehicles yet/i).first();
    this.addDialog = page.locator("dialog").filter({
      has: page.getByText(/add vehicle/i),
    });
    this.deleteDialog = page.locator("dialog").filter({
      has: page.getByText(/delete vehicle/i),
    });
  }

  public async goto() {
    await super.goto("/vehicles");
    await this.page.waitForURL(/\/vehicles/, { timeout: 15_000 });
  }

  public async getVehicleNames(): Promise<string[]> {
    const links = this.page.locator('a[href^="/vehicles/"]');
    const count = await links.count();
    const names: string[] = [];
    for (let i = 0; i < count; i++) {
      names.push((await links.nth(i).textContent())?.trim() ?? "");
    }
    return names;
  }

  public async getAvailableModels(): Promise<string[]> {
    const modelButtons = this.page
      .getByRole("dialog")
      .getByRole("button")
      .filter({ hasNotText: /cancel/i });
    const count = await modelButtons.count();
    const models: string[] = [];
    for (let i = 0; i < count; i++) {
      models.push((await modelButtons.nth(i).textContent())?.trim() ?? "");
    }
    return models;
  }

  public getVehicleLink(index: number): Locator {
    return this.vehicleList.nth(index);
  }

  public async getVehicleCount(): Promise<number> {
    return this.vehicleList.count();
  }

  public getEditButton(index: number): Locator {
    return this.page.getByRole("button", { name: /edit name/i }).nth(index);
  }

  public getDeleteButton(index: number): Locator {
    return this.page.getByRole("button", { name: /delete/i }).nth(index);
  }

  public async clickAddVehicle() {
    // Click the "Add vehicle" button to trigger React state change
    await this.addVehicleButton.click();
    // Wait for Dialog's useEffect to call showModal()
    await this.page.locator("dialog[open]").waitFor({
      state: "visible",
      timeout: 10_000,
    });
  }

  public async selectModelFromDialog(modelName: string) {
    const modelButton = this.page
      .getByRole("dialog")
      .getByRole("button")
      .filter({ hasText: new RegExp(modelName, "i") });
    await modelButton.click();
  }

  public async closeAddDialog() {
    // Press Escape to trigger native dialog close, which calls
    // Dialog component's onClose callback to reset React state
    await this.page.keyboard.press("Escape");
    await this.page.locator("dialog[open]").waitFor({
      state: "hidden",
      timeout: 5000,
    });
  }

  public async confirmDelete() {
    await this.page
      .getByRole("dialog")
      .getByRole("button", { name: /delete/i })
      .first()
      .click();
  }

  public async cancelDelete() {
    await this.page
      .getByRole("dialog")
      .getByRole("button", { name: /cancel/i })
      .first()
      .click();
  }

  public async goToDashboard() {
    await this.backToDashboardLink.click();
    await this.page.waitForURL(/\/dashboard/, { timeout: 15_000 });
  }

  public async navigateToVehicle(vehicleName: string) {
    const link = this.page
      .locator('a[href^="/vehicles/"]')
      .filter({ hasText: new RegExp(vehicleName, "i") });
    await link.click();
  }
}
