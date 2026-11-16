# Générateur du format TRACFIN

`generate.py` produit `versions/<version>/spec.gen.json`, la description du format TRACFIN (ERMES, XSD « financier », import unitaire) que consomment le validateur, l'écriture XML, l'agent et le front. Rien d'autre ne décrit le format : les règles de forme viennent de là, les règles métier écrites à la main sont dans le Go de ce paquet.

Python 3, bibliothèque standard seulement. Les entrées sont des données non fiables : on les lit, on ne les exécute jamais. Toujours lancer avec `python3 -I`.

## Ranger une version

Une version du format = un dossier `versions/<version>/` :

- `inputs/` : XSD et référentiels CSV épinglés (`inputs/financier-vX.Y.Z.xsd`, `inputs/referentiels/*.csv`) ;
- `manifest.json` : `format` (`tracfin:FR:<version>`), `status` (`active`, `deprecated`, `retired`) et, pour chaque entrée, `path`, `sha256`, `encoding`, `source_url`, `retrieved_on` ;
- `constraints.json` : décisions écrites à la main (voir plus bas), chaque entrée avec un champ `source` ;
- `spec.gen.json` : résultat commité.

Les anciennes versions restent pour ouvrir, valider et exporter les brouillons qui y sont épinglés. Le code Go (règles à la main, étapes, calculs) est commun ; s'il doit diverger, il teste la version.

## Entrées

1.4.0 : XSD « financier » v1.4.0 (2026/01/21) et 22 référentiels, récupérés le 2026-10-21 sur <https://tracfin.finances.gouv.fr/Support/documentation-technique> (profession « Banque, établissement de crédit »). Les CSV sont en `windows-1252`, séparés par `;`, avec l'en-tête `code;label` : le script les décode explicitement. Aucune certification de compatibilité ERMES en conditions réelles.

## Mettre à jour ou ajouter une version

1. Créer `versions/<nouvelle version>/inputs/`, y copier le XSD et les CSV tels quels (octets inchangés).
2. Écrire `manifest.json` avec les empreintes (`shasum -a 256`), l'encodage de chaque fichier et la date de récupération.
3. Copier `constraints.json` de la version précédente et le corriger : le générateur échoue sur tout type simple absent de `type_map`, toute référence `Ref:` sans lien, tout gabarit introuvable, toute entrée périmée.
4. Régénérer (ci-dessous) et relire le diff de `spec.gen.json`.
5. Le test `TestGeneratedSpecIsUpToDate` régénère chaque version et compare octet à octet : un `spec.gen.json` périmé fait échouer la CI.

## Régénérer

```bash
cd usecases/sargen/formats/tracfin/gen
python3 -I generate.py --version 1.4.0 --out ../versions/1.4.0/spec.gen.json
```

Le script résout `versions/<version>/` depuis son propre emplacement. Sortie déterministe : clés triées, `indent=1`, fin de ligne `\n`, ni horodatage ni chemin absolu. Il échoue (code 1, message en français) sur : empreinte différente, fichier d'entrée absent du manifest, type simple absent de `type_map`, référence de liste sans lien, construction de regex XSD que Go ne lit pas (`\i`, `\c`, `\p{Is…}`, soustraction de classes), exclusion ou attribut ignoré qui ne correspond plus à rien.

## Comment chaque contrainte est construite

**Arbre.** Le XSD est lu dans l'ordre : racine `ds`, `xs:sequence` aplaties (y compris imbriquées), `xs:choice` → nœud `choice` (aucun dans le XSD 1.4.0), attributs avant les éléments. Un élément à contenu simple et attributs (`numeroIdentification`, `precisionsSuspicion`) devient un groupe : ses attributs plus un enfant `_text` pour le texte de l'élément. Un attribut est un nœud `attribute: true`, nommé sans `@` (le chemin `name[key].name` n'accepte pas `@`) : `personnes.pp[].referencesFinancieres.referenceFinanciereIban[].iban`. Gabarit d'un enfant répétable : `parent.nom[]` ; sinon `parent.nom`. La racine a le gabarit vide.

**Cardinalités.** `minOccurs`/`maxOccurs` du XSD (`unbounded` = -1). Attribut obligatoire = `min` 1.

**Types.** `constraints.json` → `type_map` donne le `FieldType` de chaque type simple (et des types complexes à contenu simple, `t_pays`…). `xs:boolean` → boolean ; `t_date` → date (pas de motif : stockage AAAA-MM-JJ, écriture JJ/MM/AAAA) ; entiers (`t_num_6`…) → integer ; `t_decimal_12` → decimal ; types à `Ref:` ou `xs:enumeration` → code ; le reste → string. Le générateur vérifie que « code » correspond bien à une liste ou une énumération, et inversement.

**Facettes.** `maxLength`/`minLength` → `max_length`/`min_length` (chaînes, en caractères). Pour les types numériques `totalDigits` → `max_length` (nombre maximal de chiffres, séparateur exclu) et `fractionDigits` → `fraction_digits`. `pattern` → `pattern`, copié tel quel (motif XSD = correspondance complète ; le validateur l'ancre). Facettes de bornes (`minExclusive`) ignorées (liste `ignored_facets`).

**Motifs et regex.** `pattern_overrides` retire le `^` initial et le `$` final de `t_num_tel`, `t_mail`, `t_iban` et `t_numCard` (XSD:80-102, 334-368) : en regex XSD ces signes sont littéraux. Hypothèse : ERMES les lit comme des ancres ; à confirmer par un import sur ERMES démo. Note : `t_numCard` contient `(?:…)`, valide en Go mais pas en regex XSD stricte. Les consommateurs doivent ancrer chaque motif eux-mêmes sous la forme `^(?:` + motif + `)$` (certains motifs sont des alternances de premier niveau). En RE2 (Go), `\d`, `\s` et `\w` sont limités à l'ASCII.

**Listes de codes.** Chaque `Ref: liste_ref_*.csv` du XSD est lié par `code_list_bindings` à un CSV (colonnes code/libellé), à une liste en ligne (`liste_ref_typemime.csv` = `application/pdf`) ou marqué `unavailable` (`liste_ref_type_equipement.csv` : la liste n'est pas publiée, toute valeur est acceptée avec un avertissement). Le nom de la liste dans `code_lists` est le nom du `Ref:` ; `Node.code_list` porte cette clé. Une énumération en ligne devient une liste nommée d'après son type simple (`t_registre`). Un type qui cite deux références (`t_pays_dromcom` : DROM-COM et pays) donne la liste d'union `liste_ref_dromcom.csv+liste_ref_pays.csv` (le premier code gagne). Le XSD cite 18 références distinctes ; `liste_ref_devises.csv` s'y ajoute (`extra`) pour les nœuds de travail.

**Pays.** `t_pays` porte un attribut facultatif `type="other"` pour un pays en texte libre ; v1 ne le gère pas (`dropped_attributes`) : le pays est toujours un code.

**Exclusions.** `excluded` retire du spec les éléments hors v1, tous `minOccurs=0` dans le XSD : `operations.operationEnCours[]`, `operationstdf`, `pieceJointe`, `context.general.complement` (déclaration complémentaire seulement) et les renvois vers des pièces jointes (`idPieceJointe`, `idPieceJointeDonneesConnexion`, `infosFinancieresFiscales` qui n'en contient que). Une exclusion obligatoire dans le XSD ou qui ne retire rien fait échouer le générateur. Conséquence : la règle « TA2/3/5 → infoPieceIdentite » (pièces jointes) n'existe pas en v1.

**Natures.** `natures` : gabarit → nature ; l'entrée la plus spécifique s'applique à tout le sous-arbre. Absent = copié.

**Références et identifiants.** `refs` : le champ devient `type: ref` avec `ref_targets`. `export_ids` : éléments et attributs d'identifiant que l'écriture XML fabrique à partir des clés internes (`personnes.*[].id`, attribut `id` des références financières) ; ils portent `export_id: true`, n'ont jamais de valeur dans le document, ne sont réclamés ni par le validateur ni proposés au LLM ou au front. Les motifs d'`export_ids` acceptent `*` pour un segment. Les champs `emetteurTypeEmetteur`, `beneficiaireTypeBeneficiaire`, `refFinanciereEmission`, `refFinanciereReception` (et les nœuds de travail `refEmission`, `refReception`) acceptent l'identifiant d'une personne ou d'un équipement financier déjà déclaré **ou** du texte libre : une entrée `refs` avec `"ref_or_text": true` les laisse de type chaîne (facettes conservées) et ajoute `ref_targets` et `ref_or_text: true`. Si la valeur est le chemin d'un élément existant dont le gabarit est une cible, le validateur exige que l'élément existe (`dangling_ref`) et l'écriture XML la remplace par l'identifiant de l'élément ; sinon c'est du texte libre, écrit tel quel.

**Valeurs par défaut.** `defaults` → `spec.defaults` : gabarit → valeur brute posée à la création. `{case_id}` est remplacé par l'identifiant du case.

**Libellés et descriptions.** `labels` : libellés français des sections. `Node.description` = le `xs:documentation` de l'élément, sinon celui de son type (les documentations génériques `Chaines de caractere de taille N` sont ignorées) ; `descriptions` remplace le texte quand il manque ou ne suffit pas.

**Nœuds de travail.** `working_nodes` ajoute `work` (non exporté) après les enfants du XSD : `work.transactions[]` et ses champs, tous `export: false`.

**Règles.** `rules` : `id`, `kind` (`required_if`, `forbidden_if`, `exactly_one`, `at_most_one`), `scope` (gabarit d'un nœud), `then`/`among`, `when` (`path`, `op`, `values`), `severity` (absent = bloquant ; `warning` pour les « à remplir seulement si » du XSD), `description`, `source`. Un champ `for_each_scope` applique la règle à plusieurs portées (`{scope}` dans les gabarits, identifiant suffixé `@<portée>`). Tous les gabarits d'une règle sont absolus ; un gabarit répété doit se trouver sous la portée, et se résout par rapport à l'instance concrète de la portée ; un gabarit sans `[]` est global. Sémantique :
- `required_if` : les champs de `then` sont obligatoires si `when` est vraie ; `forbidden_if` : interdits si `when` est vraie ;
- `exactly_one` / `at_most_one` : un seul, ou au plus un, des champs de `among` est renseigné ; si `when` est présente, la règle ne s'applique que lorsqu'elle est vraie ;
- conditions sur un chemin répété : `eq`/`in` vrai si une valeur correspond, `not_in` vrai si aucune ne correspond, `present`/`absent` selon qu'une valeur existe ; `absent` et `not_in` sont vrais quand le champ est absent.

`hand_rules` liste les règles non exprimables (`id`, `description`, `source`), écrites en Go par le validateur et absentes de `spec.gen.json`.
