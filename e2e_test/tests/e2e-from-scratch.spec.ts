import { test, expect, type Page } from "@playwright/test";
import { BackOffice } from "./backoffice";
import { FrontEnd } from "./frontend";

const backofficeUrl = "http://localhost:3002";
const backOfficeUserEmail = "admin@checkmarble.com";
const frontendUrl = "http://localhost:3000";
const frontEndUserEmail = "bendertherobottester@futurama";

test("everything from scratch", async ({ page: backOfficePage, context }) => {
  await backOfficePage.goto(backofficeUrl);

  await BackOffice.loginInBackoffice(backOfficePage, backOfficeUserEmail);

  const e2eOrganizationName = "test e2e";

  if (
    await BackOffice.organizationExists(backOfficePage, e2eOrganizationName)
  ) {
    await BackOffice.deleteOrganization(backOfficePage, e2eOrganizationName);
  }

  await BackOffice.createOrganization(backOfficePage, e2eOrganizationName);

  await BackOffice.createDemoScenarios(backOfficePage, e2eOrganizationName);

  await BackOffice.addAdminUserToOrganization(
    backOfficePage,
    frontEndUserEmail,
    e2eOrganizationName
  );

  await BackOffice.navigateToIteration(backOfficePage, e2eOrganizationName);

  // let's go to frontend
  const frontendPage = await context.newPage();
  await frontendPage.goto(frontendUrl);
  await FrontEnd.loginInFrontend(frontendPage, frontEndUserEmail);

  await FrontEnd.createScenario(frontendPage);

  // not necessary
  await BackOffice.deleteOrganization(backOfficePage, e2eOrganizationName);
});
