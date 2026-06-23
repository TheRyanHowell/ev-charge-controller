import { Page, Locator } from "@playwright/test";
import { BasePage } from "./basePage";

export class LoginPage extends BasePage {
  public readonly emailInput: Locator;
  public readonly passwordInput: Locator;
  public readonly submitButton: Locator;

  constructor(page: Page) {
    super(page);
    this.emailInput = page.getByLabel("Email", { exact: true }).first();
    this.passwordInput = page.getByLabel("Password", { exact: true }).first();
    this.submitButton = page.getByRole("button", { name: /log[iI]n/ }).first();
  }

  public async navigateTo() {
    await super.goto("/login");
  }

  public async fillCredentials(email: string, password: string) {
    await this.emailInput.fill(email);
    await this.passwordInput.fill(password);
  }

  public async submit() {
    await this.submitButton.click();
  }

  public async login(email: string, password: string) {
    await this.navigateTo();
    await this.fillCredentials(email, password);
    await this.submit();
  }
}
