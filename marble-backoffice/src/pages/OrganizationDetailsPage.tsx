import { useState } from "react";
import { useParams } from "react-router";
import { useNavigate } from "react-router-dom";
import Typography from "@mui/material/Typography";
import Button from "@mui/material/Button";
import Box from "@mui/material/Box";
import EditIcon from "@mui/icons-material/Edit";
import AddIcon from "@mui/icons-material/Add";
import DeleteForever from "@mui/icons-material/DeleteForever";
import Stack from "@mui/material/Stack";
import { type LoadingDispatcher, useLoading } from "@/hooks/Loading";
import services from "@/injectServices";
import {
  useOrganization,
  useScenarios,
  useUsers,
  useCreateUser,
  useDeleteOrganization,
  useDecisions,
  useMarbleApiWithClientRoleApiKey,
  useDataModel,
  useEditDataModel,
} from "@/services";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import AlertDialog from "@/components/AlertDialog";
import AddUserDialog from "@/components/AddUserDialog";
import { type CreateUser, Role, PageLink } from "@/models";
import ListOfUsers from "@/components/ListOfUsers";
import IconButton from "@mui/material/IconButton";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import TextareaAutosize from "@mui/base/TextareaAutosize";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import Paper from "@mui/material/Paper";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Container from "@mui/material/Container";
import ListOfScenarios from "@/components/ListOfScenarios";
import ReactJson from "react-json-view";
import NorthEastIcon from "@mui/icons-material/NorthEast";
import Alert from "@mui/material/Alert";

function OrganizationDetailsPage() {
  const { organizationId } = useParams();
  const navigate = useNavigate();

  if (!organizationId) {
    throw Error("Organization Id is missing");
  }

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { organization } = useOrganization(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId
  );

  const [deleteOrgAlertDialogOpen, setDeleteOrgAlertDialogOpen] =
    useState(false);
  const { deleteOrganization } = useDeleteOrganization(
    services().organizationService
  );

  const handleDeleteOrgClick = () => {
    setDeleteOrgAlertDialogOpen(true);
  };
  const handleDeleteOrg = async () => {
    await deleteOrganization(organizationId);
    setDeleteOrgAlertDialogOpen(false);
    navigate(PageLink.Organizations);
  };

  const handleNavigateToEdit = () => {
    navigate(PageLink.organizationEdit(organizationId));
  };

  const handleNavigateToIngestion = () => {
    navigate(PageLink.ingestion(organizationId));
  };

  const handleNavigateToDecisions = () => {
    navigate(PageLink.decisions(organizationId));
  };

  const handleNavigateToOrganizations = () => {
    navigate(PageLink.Organizations);
  };

  const [tabValue, setTabValue] = useState(0);

  const handleChange = (_: React.SyntheticEvent, newValue: number) => {
    setTabValue(newValue);
  };

  return (
    <>
      {/* Dialog: Delete organisation */}
      <AlertDialog
        title="Confirm organization deletion"
        open={deleteOrgAlertDialogOpen}
        handleClose={() => {
          setDeleteOrgAlertDialogOpen(false);
        }}
        handleValidate={handleDeleteOrg}
      >
        <Typography variant="body1">
          Are you sure to delete {organization?.name} ? This action is
          destructive (no soft delete)
        </Typography>
      </AlertDialog>

      {/* Main layout */}
      <DelayedLinearProgress loading={pageLoading} />

      <Container sx={{ my: 1 }}>
        <Stack
          direction="column"
          justifyContent="flex-start"
          alignItems="center"
          spacing={2}
        >
          {/* Page content header */}
          <Stack
            direction="row"
            justifyContent="space-between"
            alignItems="center"
            spacing={2}
            sx={{
              minWidth: "100%",
            }}
          >
            {/* Title */}
            <Stack
              direction="row"
              justifyContent="flex-start"
              alignItems="center"
              spacing={2}
            >
              <IconButton
                aria-label="back"
                onClick={handleNavigateToOrganizations}
              >
                <ArrowBackIcon />
              </IconButton>
              <Typography variant="h3">{organization?.name}</Typography>
            </Stack>

            {/* Organization Actions */}
            <Stack
              direction="row"
              justifyContent="flex-start"
              alignItems="center"
              spacing={2}
            >
              <Button
                onClick={handleNavigateToDecisions}
                variant="outlined"
                startIcon={<NorthEastIcon />}
              >
                Decisions
              </Button>

              <Button
                onClick={handleNavigateToIngestion}
                variant="outlined"
                startIcon={<NorthEastIcon />}
              >
                Ingestion
              </Button>

              <Button
                onClick={handleNavigateToEdit}
                variant="contained"
                startIcon={<EditIcon />}
              >
                Edit
              </Button>

              <Button
                onClick={handleDeleteOrgClick}
                variant="contained"
                startIcon={<DeleteForever />}
                color="error"
              >
                Delete
              </Button>
            </Stack>
          </Stack>

          {/* Page content details */}
          {organization && (
            <Paper sx={{ minWidth: "100%", p: 2, fontSize: "0.8em" }}>
              <ReactJson
                src={organization}
                name="organization"
                collapsed={1}
                theme={"rjv-default"}
              />
            </Paper>
          )}

          <Paper sx={{ minWidth: "100%" }}>
            <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
              <Tabs
                value={tabValue}
                onChange={handleChange}
                aria-label="basic tabs example"
              >
                <Tab label="Users" />
                <Tab label="Scenarios" />
                <Tab label="Decisions" />
                <Tab label="Data Model" />
              </Tabs>
            </Box>

            <Box sx={{ p: 3 }}>
              {tabValue === 0 && (
                <OrganizationDetailsUserList
                  organizationId={organizationId}
                  pageLoadingDispatcher={pageLoadingDispatcher}
                />
              )}
              {tabValue === 1 && (
                <OrganizationDetailsScenariosList
                  organizationId={organizationId}
                  pageLoadingDispatcher={pageLoadingDispatcher}
                />
              )}
              {tabValue === 2 && (
                <OrganizationDetailsDecisionsList
                  organizationId={organizationId}
                  pageLoadingDispatcher={pageLoadingDispatcher}
                />
              )}
              {tabValue === 3 && (
                <OrganizationDetailsDataModel
                  organizationId={organizationId}
                  pageLoadingDispatcher={pageLoadingDispatcher}
                />
              )}
            </Box>
          </Paper>
        </Stack>
      </Container>
    </>
  );
}

function OrganizationDetailsUserList({
  pageLoadingDispatcher,
  organizationId,
}: {
  pageLoadingDispatcher: LoadingDispatcher;
  organizationId: string;
}) {
  const navigate = useNavigate();

  const { users, refreshUsers } = useUsers(
    services().userService,
    pageLoadingDispatcher,
    organizationId
  );

  const [createUserDialogOpen, setCreateUserDialogOpen] = useState(false);
  const { createUser } = useCreateUser(services().userService);

  const handleCreateUserClick = () => {
    setCreateUserDialogOpen(true);
  };

  const handleValidateCreateUser = async (newUser: CreateUser) => {
    await createUser(newUser);
    await refreshUsers();
  };

  return (
    <>
      {/* Dialog: Add user */}
      <AddUserDialog
        open={createUserDialogOpen}
        setDialogOpen={setCreateUserDialogOpen}
        onValidate={handleValidateCreateUser}
        organizationId={organizationId}
        availableRoles={[Role.VIEWER, Role.BUILDER, Role.PUBLISHER, Role.ADMIN]}
        title="Add User"
      />

      <Stack
        direction="column"
        justifyContent="flex-start"
        alignItems="center"
        spacing={2}
      >
        <Stack
          direction="row"
          justifyContent="flex-end"
          alignItems="center"
          spacing={2}
          sx={{
            minWidth: "100%",
          }}
        >
          <Button
            onClick={handleCreateUserClick}
            variant="contained"
            startIcon={<AddIcon />}
          >
            Add User
          </Button>
        </Stack>

        <Box sx={{ minWidth: "100%" }}>
          {users != null && (
            <ListOfUsers
              users={users}
              onUserDetailClick={(userId) => {
                navigate(PageLink.userDetails(userId));
              }}
            />
          )}
        </Box>
      </Stack>
    </>
  );
}

function OrganizationDetailsScenariosList({
  organizationId,
  pageLoadingDispatcher,
}: {
  pageLoadingDispatcher: LoadingDispatcher;
  organizationId: string;
}) {
  const { scenarios } = useScenarios(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId
  );

  if (scenarios == null || scenarios.length == 0) {
    return <Typography variant="subtitle1">No scenarios</Typography>;
  } else return <ListOfScenarios scenarios={scenarios} />;
}

function OrganizationDetailsDecisionsList({
  pageLoadingDispatcher,
  organizationId,
}: {
  organizationId: string;
  pageLoadingDispatcher: LoadingDispatcher;
}) {
  const { marbleApiWithClientRoleApiKey } = useMarbleApiWithClientRoleApiKey(
    services().apiKeyService,
    pageLoadingDispatcher,
    organizationId
  );

  const { decisions } = useDecisions(
    marbleApiWithClientRoleApiKey,
    pageLoadingDispatcher
  );

  if (decisions == null) {
    return <Typography variant="subtitle1">No decisions</Typography>;
  } else
    return (
      <>
        <Typography variant="subtitle1">
          {decisions?.length} Decisions
        </Typography>
        <List>
          {(decisions || []).map((decision) => (
            <ListItem key={decision.decisionId}>
              <ListItemText primary={decision.decisionId} />
            </ListItem>
          ))}
        </List>
      </>
    );
}

function OrganizationDetailsDataModel({
  pageLoadingDispatcher,
  organizationId,
}: {
  pageLoadingDispatcher: LoadingDispatcher;
  organizationId: string;
}) {
  const { dataModel } = useDataModel(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId
  );

  const {
    dataModelString,
    setDataModelString,
    saveDataModel,
    saveDataModelConfirmed,
    dataModelError,
    saveDataModelAlertDialogOpen,
    setSaveDataModelAlertDialogOpen,
    canSave,
  } = useEditDataModel(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId,
    dataModel
  );

  return dataModelString ? (
    <>
      {/* Dialog: Replace Data Nodel */}
      <AlertDialog
        title="Confirm organization deletion"
        open={saveDataModelAlertDialogOpen}
        handleClose={() => {
          setSaveDataModelAlertDialogOpen(false);
        }}
        handleValidate={saveDataModelConfirmed}
      >
        <Typography variant="body1">
          Are you sure to replace the Data Model ? This action is destructive:
          all the ingested data of this organization will be erased.
        </Typography>
      </AlertDialog>
      {dataModelString !== null && (
        <Box sx={{
          mb: 4,
        }}>
          <TextareaAutosize
            minRows="5"
            value={dataModelString}
            style={{ width: "100%" }}
            onChange={(e) => setDataModelString(e.target.value)}
          />
          {dataModelError && <Alert severity="error">{dataModelError}</Alert>}
          <Button
            onClick={saveDataModel}
            variant="contained"
            startIcon={<DeleteForever />}
            color="warning"
            disabled={!canSave}
          >
            Replace Data Model
          </Button>
        </Box>
      )}
    </>
  ) : (
    false
  );
}

export default OrganizationDetailsPage;
