import services from "@/injectServices";
import { Organization } from "@/models";
import { useAllOrganizations } from "@/services";
import BusinessIcon from "@mui/icons-material/Business";
import AddIcon from "@mui/icons-material/Add";
import ListSubheader from "@mui/material/ListSubheader";
import Fab from "@mui/material/Fab";
import Avatar from "@mui/material/Avatar";
import Container from "@mui/material/Container";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemAvatar from "@mui/material/ListItemAvatar";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";

function OrganizationsPage() {
  const { allOrganizations } = useAllOrganizations(
    services().organizationService
  );

  const fakeOrganizations: Organization[] = [
    {
      organizationId: "someid",
      name: "Zorg",
      dateCreated: new Date(),
    },
  ];

  return (
    <Container
      sx={{
        maxWidth: "md",
        position:'relative'
      }}
    >
      <Fab
        sx={{ position: "absolute", top: "10px", right:"50px", paddingRight:"20px"}}
        color="primary"
        size="small"
        variant="extended"
        aria-label="add"
      >
        <AddIcon sx={{ mr: 1 }} />
        New Organization
      </Fab>
      <List aria-label="organizations">
        <ListSubheader inset>
          {allOrganizations?.length} Organizations
        </ListSubheader>
        {(allOrganizations || fakeOrganizations).map((organization) => (
          <ListItem key={organization.organizationId}>
            <ListItemButton>
              <ListItemAvatar>
                <Avatar>
                  <BusinessIcon />
                </Avatar>
              </ListItemAvatar>
              <ListItemText
                primary={organization.name}
                secondary={organization.dateCreated.toDateString()}
              />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </Container>
  );
}

export default OrganizationsPage;
