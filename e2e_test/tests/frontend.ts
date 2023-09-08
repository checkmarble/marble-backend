import { type Page, expect } from "@playwright/test";

export namespace FrontEnd {
  export async function loginInFrontend(page: Page, userName: string) {
    await expect(page.getByText("Login to your account")).toBeVisible();

    const popupPromise = page.waitForEvent("popup");
    await page.getByRole("button", { name: "Sign in with Google" }).click();
    const popup = await popupPromise;
    await popup.waitForLoadState(); // Wait for the popup to load.
    // await expect(popup.getByText("Sign-in with Google.com")).toBeVisible();
    await popup.getByText(userName).click();
    await page.waitForLoadState();
  }

  export async function createScenario(page: Page) {
    await page.getByRole("button", { name: "New Scenario" }).click();
    await page.getByPlaceholder("Add a name").fill("test scenario");
    await page.getByPlaceholder("Add a description").fill("test scenario");

    await page.getByRole("combobox").click();
    // await page.getByLabel("Select a trigger object").click();
    await page.getByRole("option", { name: "transactions" }).click();

    await page.getByRole("button", { name: "Save" }).click();

    // await expect(page.getByText("How to run this scenario ?")).toBeVisible();

    const createTrigger = async () => {
      await page.getByText("Trigger", { exact: true }).click();
      // scroll to save
      const saveButton = page.getByText("Save");
      await saveButton.scrollIntoViewIfNeeded();

      // left operand
      await page.getByText("Condition", { exact: true }).click();
      await page.getByLabel("left-operand").click();

      await page.getByRole("textbox").fill("account.balance");
      await page.getByRole("button", { name: /^account.balance/ }).click();

      // operator combobox
      await page.locator("button").filter({ hasText: "..." }).click();

      // combo box is too small: hover on the small arrow to find the "="
      page.getByRole("option", { name: ">=" });

      const list = page.getByRole("listbox");
      await list.press("=");
      await list.getByText("=", { exact: true }).click();

      // right operand
      await page.getByLabel("right-operand").click();
      await page.getByRole("textbox").fill("account.balance");
      await page.getByRole("button", { name: /^account.balance/ }).click();

      await saveButton.click();
    };

    const createRule = async () => {
      await page.getByText("Rules", { exact: true }).click();
      await page.getByText("New Rule", { exact: true }).click();

      // await page.getByPlaceholder("Add a name to your rule").fill("test rule");
      // await page
      //   .getByPlaceholder("Add a description to your scenario")
      //   .fill("test rule description");
      // await page
      //   .getByPlaceholder("Add a score modifier to your rule")
      //   .fill("10");
      // await page.getByRole("button", { name: "Create a new rule" }).click();

      // waiting for the model to close
      // await expect(
      //   page.getByRole("heading", { name: "New Rule" })
      // ).not.toBeVisible();

      // page too long, scroll to the bottom
      // await page
      //   .locator("button")
      //   .filter({ hasText: "Delete this rule" })
      //   .scrollIntoViewIfNeeded();

      await page
        .getByText("New rule", { exact: true })
        .fill("test rule name");
      await page
        .getByPlaceholder("Edit the description of your rule", { exact: true })
        .fill("test rule description");

      await page.getByText("or", { exact: true }).click();

      // left operand
      await page
        .locator('[id="headlessui-combobox-input-\\:r5h\\:"]')
        .fill("account.balance");
      await page.getByText("account.balance", { exact: true }).click();

      // operator
      await page
        .locator("div")
        .filter({ hasText: /^requiredrequired$/ })
        .locator("button")
        .click();
      await page.locator('[id="radix-\\:r5l\\:"] > div:nth-child(3)').hover();
      await page.getByText(">", { exact: true }).click();

      // right operand
      await page
        .locator('[id="headlessui-combobox-input-\\:r5n\\:"]')
        .fill("1000");
      await page.getByText("1000", { exact: true }).click();

      await page.getByText("Save").nth(1).click();

      // for some reason, we must go back
      await page.getByRole("main").getByRole("link").click();
    };

    const configureDecision = async () => {
      await page.getByText("Decision", { exact: true }).click();

      await page.locator('input[name="thresholds\\.0"]').fill("10");
      await page.locator('input[name="thresholds\\.1"]').fill("100");

      await page.getByText("Save", { exact: true }).click();
    };

    await createTrigger();
    await createRule();
    await configureDecision();
  }
}
