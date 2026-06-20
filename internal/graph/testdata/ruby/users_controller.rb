class UsersController < ApplicationController
  def index
    @users = User.all
    render json: @users
  end

  def create
    user = build_user
    if user.save
      render json: user, status: :created
    else
      render json: user.errors, status: :unprocessable_entity
    end
  end

  private

  def build_user
    User.new(user_params)
  end

  def user_params
    params.require(:user).permit(:name, :email)
  end
end
