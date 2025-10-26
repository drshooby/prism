import type { Type } from "./types";

/**
 * Configuration Representation — A sub-object of plan output that describes a parsed Terraform configuration.
 */
export interface Configuration {
  /**
   * "provider_configs" describes all of the provider configurations throughout the configuration tree, flattened into a single map for convenience since provider configurations are the one concept in Terraform that can span across module boundaries.
   *
   * Keys in the provider_configs map are to be considered opaque by callers, and used just for lookups using the "provider_config_key" property in each resource object.
   */
  provider_config: Record<string, ProviderConfig>;

  /** "root_module" describes the root module in the configuration, and serves as the root of a tree of similar objects describing descendant modules. */
  root_module: ModuleConfiguration;
}

/** Provider configuration entry. */
export interface ProviderConfig {
  /** Name of the provider without alias (e.g., "aws"). */
  name: string;

  /** Fully-qualified provider name (e.g., "registry.terraform.io/hashicorp/aws"). */
  full_name: string;

  /** Alias for a non-default configuration, or unset for a default configuration. */
  alias?: string;

  /**
   * "module_address" is included only for provider configurations that are declared in a descendant module, and gives the opaque address for the module that contains the provider configuration.
   */
  module_address?: string;

  /** "expressions" describes the provider-specific content of the configuration block, as a Block Expressions Representation. */
  expressions?: BlockExpressions;
}

/** The structure of a module in configuration (root or nested). */
export interface ModuleConfiguration {
  /** "outputs" describes the output value configurations in the module. */
  outputs?: Record<string, { expression: Expression; sensitive: boolean }>;

  /** Resources and data blocks in the module configuration. */
  resources?: ConfigurationResource[];

  /**
   * "module_calls" describes the "module" blocks in the module. During evaluation, a module call with count or for_each may expand to multiple module instances, but in configuration only the block itself is represented.
   *
   * The key is the module call name chosen in the configuration.
   */
  module_calls?: Record<string, ModuleCall>;
}

/** A resource or data block entry within configuration. */
export interface ConfigurationResource {
  /** Opaque absolute address for the resource (e.g., "aws_instance.example"). */
  address: string;

  /** Mode: "managed" or "data". */
  mode: "managed" | "data";

  /** Resource type e.g. "aws_instance". */
  type: string;

  /** Local name used. */
  name: string;

  /**
   * "provider_config_key" is the key into "provider_configs" for the provider configuration that this resource is associated with. If the provider configuration was passed into this module from the parent module, the key will point to the original provider config block.
   */
  provider_config_key?: string;

  /** "provisioners" is an optional field which describes any provisioners. Connection info will not be included here. */
  provisioners?: { type: string; expressions?: BlockExpressions }[];

  /** Resource-type-specific content of the configuration block. */
  expressions?: BlockExpressions;

  /** Schema version number indicated by the provider for the type-specific arguments described in "expressions". */
  schema_version: number;

  /**
   * "count_expression" and "for_each_expression" describe the expressions given for the corresponding meta-arguments in the resource configuration block. These are omitted if the corresponding argument isn't set.
   */
  count_expression?: Expression;
  for_each_expression?: Expression;
}

/** Representation of a module call (a `module` block in configuration). */
export interface ModuleCall {
  /**
   * "resolved_source" is the resolved source address of the module, after any normalization and expansion. This could be either a go-getter-style source address or a local path starting with "./" or "../". If the user gave a registry source address then this is the final location of the module as returned by the registry, after following any redirect indirection.
   */
  resolved_source?: string;

  /** The expressions for the arguments within the block that correspond to input variables in the child module. */
  expressions?: BlockExpressions;

  /**
   * "count_expression" and "for_each_expression" describe the expressions given for the corresponding meta-arguments in the module configuration block. These are omitted if the corresponding argument isn't set.
   */
  count_expression?: Expression;
  for_each_expression?: Expression;

  /** "module" is a representation of the configuration of the child module itself, using the same structure as the "root_module" object, recursively describing the full module tree. */
  module?: ModuleConfiguration;
}

/* -------------------------------------------------------------------------- */
/* Expressions                                                                */
/* -------------------------------------------------------------------------- */

/**
 * Expression Representation — A sub-object of a configuration representation that describes an unevaluated expression.
 */
export interface Expression {
  /**
   * "constant_value" is set only if the expression contains no references to other objects, in which case it gives the resulting constant value. This is mapped as for the individual values in a value representation.
   */
  constant_value?: Type;

  /**
   * Alternatively, "references" will be set to a list of references in the expression. Multi-step references will be unwrapped and duplicated for each significant traversal step, allowing callers to more easily recognize the objects they care about without attempting to parse the expressions. Callers should only use string equality checks here, since the syntax may be extended in future releases.
   */
  references?: string[];
}

/**
 * Block Expressions Representation — A sub-object of a configuration representation that describes the expressions nested inside a block.
 *
 * In some cases, it is the entire content of a block (possibly after certain special arguments have already been handled and removed) that must be represented. For that, we have an <block-expressions-representation> structure.
 */
export interface BlockExpressions {
  [attribute: string]:
    | Expression
    | BlockExpressions
    | Expression[]
    | BlockExpressions[];
}
