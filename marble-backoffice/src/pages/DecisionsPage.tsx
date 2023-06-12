import { useParams } from "react-router";
import Container from "@mui/system/Container";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import services from "@/injectServices";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { useLoading } from "@/hooks/Loading";
import {
  useDecisions,
  useScenarios,
  useCreateDecision,
  useMarbleApiWithClientRoleApiKey,
  useOrganization,
} from "@/services";
import Button from "@mui/material/Button";
import InputLabel from "@mui/material/InputLabel";
import Select, { SelectChangeEvent } from "@mui/material/Select";
import MenuItem from "@mui/material/MenuItem";
import { Box } from "@mui/system";

function DecisionsPage() {
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

  const { marbleApiWithClientRoleApiKey } = useMarbleApiWithClientRoleApiKey(
    services().apiKeyService,
    pageLoadingDispatcher,
    organizationId
  );

  const { decisions, refreshDecisions } = useDecisions(
    marbleApiWithClientRoleApiKey,
    pageLoadingDispatcher
  );

  const {
    createDecision,
    createScenarioViewModel,
    setCreateScenarioViewModel,
    createDecisionformValid,
  } = useCreateDecision(
    marbleApiWithClientRoleApiKey,
    pageLoadingDispatcher,
    refreshDecisions
  );

  const handleScenarioIdChange = (event: SelectChangeEvent<string>) => {
    const scenarioId = event.target.value;
    setCreateScenarioViewModel({
      ...createScenarioViewModel,
      scenarioId: scenarioId,
    });
  };

  const handleCreateDecision = async () => {
    await createDecision();
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
                Decisions of {organization.name}
              </Typography>
            </>
          )}
        </Paper>
        <List>
          {(decisions || []).map((decision) => (
            <ListItem key={decision.decisionId}>
              <ListItemText primary={decision.decisionId} />
            </ListItem>
          ))}
        </List>

        <Paper sx={{ m: 2, p: 2 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "flex-start",
              flexDirection: "column",
              gap: 2,
            }}
          >
            <InputLabel id="select-scenario-label">Scenarios</InputLabel>
            <Select
              labelId="select-scenario-label"
              id="select-scenario-select"
              value={createScenarioViewModel.scenarioId}
              variant="standard"
              label="Role"
              disabled={scenarios === null}
              onChange={handleScenarioIdChange}
            >
              {(scenarios || []).map((scenario) => (
                <MenuItem key={scenario.scenarioId} value={scenario.scenarioId}>
                  {scenario.name} {scenario.scenarioId}
                </MenuItem>
              ))}
            </Select>

            <Button
              disabled={!createDecisionformValid}
              onClick={handleCreateDecision}
              variant="outlined"
            >
              Create Decision
            </Button>
          </Box>
        </Paper>
      </Container>
    </>
  );
}

export default DecisionsPage;
