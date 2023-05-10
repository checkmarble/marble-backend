export class SignInError extends Error {
  from: Error;

  constructor(message: string, from: Error) {
    super(message, { cause: from });
    this.from = from;
  }
}
