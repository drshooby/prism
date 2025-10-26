import type { Values } from "./values";

/**
 * State Representation — The complete top-level object returned by `terraform show -json <STATE FILE>`.
 *
 * State does not have any significant metadata not included in the common values representation, so the `<state-representation>` uses the following format:
 */
export interface State {
  /**
   * A Values Representation object derived from the values in the state. Because the state is always fully known, this is always complete.
   */
  values: Values;

  /**
   * Terraform CLI version that produced the state (string).
   */
  terraform_version: string;
}
