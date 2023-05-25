import Container from "@mui/system/Container";
import { useLoading } from "@/hooks/Loading";
import services from "@/injectServices";
import { useOrganization } from "@/services";
import { useParams } from "react-router";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { Typography } from "@mui/material";

function OrganizationDetailsPage() {
  const { organisationId } = useParams();

  if (!organisationId) {
    throw Error("Organization Id is missing");
  }

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { organization } = useOrganization(
    services().organizationService,
    pageLoadingDispatcher,
    organisationId
  );
  return (
    <>
      <DelayedLinearProgress loading={pageLoading} />
      <Container>
        {/* <div>organisationId: {organisationId}</div> */}
        {organization && (
          
          <>
          <Typography component="h1" variant="h4">
            {organization.name}
          </Typography>
          <Typography variant="body1">
            Nothing to brag about
          </Typography>
          
          
          </>
        )}
      </Container>
    </>
  );
}

export default OrganizationDetailsPage;
