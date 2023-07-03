export interface DataAccessesIdentifier {
  varName: string;
  //   Vartype: string | null;
  //   Optional: boolean;
}

export interface BuilderIdentifiers {
  dataAccesses: DataAccessesIdentifier[];
}
