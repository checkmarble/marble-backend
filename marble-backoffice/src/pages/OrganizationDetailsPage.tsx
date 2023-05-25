import Container from "@mui/system/Container";
import { useLoading } from "@/hooks/Loading";
import services from "@/injectServices";
import { useOrganization, useScenarios } from "@/services";
import { useParams } from "react-router";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Typography from "@mui/material/Typography";

function OrganizationDetailsPage() {
  const { organizationId } = useParams();

  if (!organizationId) {
    throw Error("Organization Id is missing");
  }

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { organization } = useOrganization(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId
  );

  const { scenarios } = useScenarios(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId
  );
  return (
    <>
      <DelayedLinearProgress loading={pageLoading} />
      <Container>
        {/* <div>organizationId: {organizationId}</div> */}
        {organization && (
          <>
            <Typography variant="h3">{organization.name}</Typography>
          </>
        )}
        {scenarios != null && (
          <>
            <Typography variant="h4">{scenarios.length} Scenarios</Typography>
            {scenarios.map((scenario) => (
              <Card key={scenario.scenariosId}>
                <CardContent>
                  <Typography
                    sx={{ fontSize: 14 }}
                    color="text.secondary"
                    gutterBottom
                  >
                    Scenario
                  </Typography>
                  <Typography variant="h5" component="div">
                    {scenario.name}
                  </Typography>
                  <Typography sx={{ mb: 1.5 }} color="text.secondary">
                    {scenario.createdAt.toDateString()}
                  </Typography>
                  <Typography variant="body2">
                    {scenario.description}
                  </Typography>
                </CardContent>
              </Card>
            ))}
          </>
        )}
      </Container>
    </>
  );
}

export default OrganizationDetailsPage;
