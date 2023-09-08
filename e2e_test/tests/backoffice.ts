import { expect, type Page } from "@playwright/test";

export namespace BackOffice {
    export async function loginInBackoffice(page: Page, userName: string) {
      await page.getByTestId("login-button").click();
  
      await expect(page.getByText("Sign-in with Google.com")).toBeVisible();
      await page.waitForURL("http://localhost:9099/emulator/auth/handler?**");
      // await page.getByText("Jean-Baptiste Emanuel Zorg").click();
      await page.getByText(userName).click();
      await page.waitForLoadState();
      // await expect(page.getByText("Your credentials")).toBeVisible();
    }
  
    export async function organizationExists(
        page: Page,
        organizationName: string
    ) : Promise<boolean> {
        await page.getByRole("button", { name: "Organizations" }).click();
        return await page.getByRole("row", { name: organizationName }).isVisible();
    }
    
    export async function createOrganization(
      page: Page,
      organizationName: string
    ) {
      await page.getByRole("button", { name: "Organizations" }).click();
      await page.getByRole("button", { name: "New Organisation" }).click();
  
      await page.getByLabel("Organization name").fill(organizationName);
      await page.getByRole("button", { name: "Create" }).click();
      await expect(page.getByRole('heading', { name: 'Create Organization' })).not.toBeVisible();
    }
  
    async function navigateToOrganizationDetails(
      page: Page,
      organizationName: string
    ) {
      // navigate to test organization page and create demo scenario
      await page
        .getByRole("row", { name: organizationName })
        .getByLabel("Details")
        .click();
    }
  
    export async function createDemoScenarios(
      page: Page,
      organizationName: string
    ) {
      await navigateToOrganizationDetails(page, organizationName);
      await page.getByRole("tab", { name: "Scenarios" }).click();
      await page.getByRole("button", { name: "Add Demo Scenario" }).click();
      // there is no ui feedback
      await page.waitForLoadState("networkidle");
      await page.getByLabel("back").click();
    }
  
    export async function addAdminUserToOrganization(
      page: Page,
      userName: string,
      organizationName: string
    ) {
      await navigateToOrganizationDetails(page, organizationName);
      await page.getByRole("tab", { name: "Users" }).click();
      await page.getByRole("button", { name: "Add User" }).click();
      await page.getByLabel("User's email").fill(userName);
      await page.getByLabel("Role").click();
      await page.getByRole("option", { name: "admin" }).click();
      await page.getByRole("button", { name: "Create" }).click();
      await page.getByLabel("back").click();
    }
  
    export async function navigateToIteration(
      page: Page,
      organizationName: string
    ) {
      await navigateToOrganizationDetails(page, organizationName);
      await page.getByRole('tab', { name: 'Scenarios' }).click();

      // navigate to scenario details for no reason
      await page
        .getByRole("row", { name: "Demo scenario" })
        .getByLabel("Details")
        .click();
      await page.getByRole("button", { name: "Live (version 1)" }).click();
  
      await expect(page.getByText("Live Iteration (version 1)")).toBeVisible();
    }
  
    export async function deleteOrganization(
      page: Page,
      organizationName: string
    ) {
      await page.getByRole("button", { name: "Organizations" }).click();
      await page
        .getByRole("row", { name: organizationName })
        .getByLabel("Details")
        .click();
      await page.getByRole("button", { name: "Delete" }).click();
      await page.getByRole("button", { name: "Validate" }).click();
      await expect(
        page.getByRole('heading', { name: 'Confirm organization deletion' })
      ).not.toBeVisible();
    }
  }
  