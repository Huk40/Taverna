export type CharacterConfig = {
  body: string;
  head: string;
  hair: string;
  armor: string;
  weapon: string;
  pose: string;
  base: string;
  scale_mm: number;
  colors: {
    armor: string;
    hair: string;
    cape: string;
  };
};

export type Project = {
  id: string;
  name: string;
  config: CharacterConfig;
  created_at: string;
  updated_at: string;
};
