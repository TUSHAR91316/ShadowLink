require 'xcodeproj'
project_path = 'shadowlink_gui/ios/Runner.xcodeproj'
project = Xcodeproj::Project.open(project_path)
target = project.targets.first
framework_group = project.main_group.find_subpath('Frameworks', true)
file_ref = framework_group.new_reference('Mobile.xcframework')
target.frameworks_build_phase.add_file_reference(file_ref, true)

embed_phase = target.build_phases.find { |p|
  p.is_a?(Xcodeproj::Project::Object::PBXCopyFilesBuildPhase) && p.name == 'Embed Frameworks'
}
if embed_phase.nil?
  embed_phase = project.new(Xcodeproj::Project::Object::PBXCopyFilesBuildPhase)
  embed_phase.name = 'Embed Frameworks'
  embed_phase.symbol_dst_subfolder_spec = :frameworks
  target.build_phases << embed_phase
end
project.save
