import PersonIcon from "@mui/icons-material/Person";
import ListSubheader from "@mui/material/ListSubheader";
import Avatar from "@mui/material/Avatar";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemAvatar from "@mui/material/ListItemAvatar";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import { User } from "@/models";

interface ListOfUsersProps {
  users: User[];
  onUserClick?: (user: User) => void;
}

export default function ListOfUsers(props: ListOfUsersProps) {
  const users = props.users;

  return (
    <>
      <List aria-label="users">
        <ListSubheader inset>{users.length} Users</ListSubheader>
        {users.map((user) => (
          <ListItem key={user.userId}>
            <ListItemButton
              onClick={() => {
                props.onUserClick?.(user);
              }}
            >
              <ListItemAvatar>
                <Avatar>
                  <PersonIcon />
                </Avatar>
              </ListItemAvatar>
              <ListItemText primary={user.email} secondary={user.role} />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </>
  );
}
