import { useParams } from "react-router";
import Container from "@mui/system/Container";
import Typography from "@mui/material/Typography";
import Paper from "@mui/material/Paper";
import Button from "@mui/material/Button";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { useLoading } from "@/hooks/Loading";
import services from "@/injectServices";
import {
  useOrganization,
  useIngestion,
  useMarbleApiWithClientRoleApiKey,
} from "@/services";
import { useState } from "react";
import { IngestObject } from "@/infra/MarbleApi";

function IngestionPage() {
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

  const { apiKey, marbleApiWithClientRoleApiKey } =
    useMarbleApiWithClientRoleApiKey(
      services().apiKeyService,
      pageLoadingDispatcher,
      organizationId
    );

  const { ingest } = useIngestion(marbleApiWithClientRoleApiKey);

  const [ingestResult, setIngestResult] = useState<IngestObject[] | null>(null);

  const handleIngest = async () => {
    setIngestResult(await ingest());
  };

  return (
    <>
      <DelayedLinearProgress loading={pageLoading} />
      <Container
        sx={{
          maxWidth: "md",
          position: "relative",
        }}
      >
        <Paper sx={{ m: 2, p: 2 }}>
          {organization && (
            <>
              <Typography variant="h4">
                Data Ingestion in {organization.name}
              </Typography>
            </>
          )}
          {apiKey && (
            <p>
              Ready to ingest data with api key: <code>{apiKey}</code>
            </p>
          )}
          <Button onClick={handleIngest} variant="outlined">
            Ingest transaction
          </Button>

          <pre>
            {ingestResult !== null
              ? JSON.stringify(ingestResult, null, 2)
              : false}
          </pre>
        </Paper>
      </Container>
    </>
  );
}

export default IngestionPage;
