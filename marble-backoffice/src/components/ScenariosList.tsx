import { useNavigate } from "react-router-dom";
import Stack from "@mui/material/Stack";
import AddIcon from "@mui/icons-material/Add";
import Button from "@mui/material/Button";
import { useScenarios, useAddScenarios } from "@/services";
import { type LoadingDispatcher } from "@/hooks/Loading";
import services from "@/injectServices";
import ListOfScenarios from "./ListOfScenarios";
import { PageLink } from "@/models";

export default function ScenariosList({
  organizationId,
  pageLoadingDispatcher,
}: {
  pageLoadingDispatcher: LoadingDispatcher;
  organizationId: string;
}) {
  const { scenarios, refreshScenarios } = useScenarios(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId
  );

  const navigate = useNavigate();

  const { addDemoScenario } = useAddScenarios(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId,
    refreshScenarios
  );

  const handleCreateDemoScenarioClick = () => {
    addDemoScenario();
  };

  return (
    <>
      <Stack
        direction="row"
        justifyContent="flex-end"
        alignItems="center"
        spacing={2}
        sx={{
          mb: 2,
        }}
      >
        <Button
          onClick={handleCreateDemoScenarioClick}
          variant="contained"
          startIcon={<AddIcon />}
        >
          Add Demo Scenario
        </Button>
      </Stack>

      <ListOfScenarios
        scenarios={scenarios}
        onScenarioDetailClick={(scenarioId) => {
          navigate(PageLink.scenarioDetailsPage(scenarioId));
        }}
      />
    </>
  );
}
