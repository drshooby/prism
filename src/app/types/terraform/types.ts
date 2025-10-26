type Primitive = string | number | boolean;

export type Type =
  | Primitive
  | Array<Primitive>
  | Set<Primitive>
  | Map<string, Type>
  | null;

export type TypeString =
  | "string"
  | "number"
  | "bool"
  | "list"
  | "set"
  | "map"
  | "null";
